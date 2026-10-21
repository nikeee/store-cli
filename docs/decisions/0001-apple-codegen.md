# ADR 0001: App Store Connect client generation

- Status: accepted
- Date: 2026-10-21
- Issue: #8

## Context

`store` calls about 70 of the 1303 operations in the App Store Connect API spec
(`api/apple/openapi.oas.json`, version 4.5.1, 1001 paths, 1473 schemas). The spec is JSON:API
shaped: `data`, `attributes`, `relationships`, `included`, `links.next`, `errors[]`. We want typed
Go code for exactly the operations we use, reproducible with `go generate`, committed, and checked
for drift in CI.

Both candidates below and the own generator run on the same pruned spec:
`tools/ascprune` keeps the operations in `api/apple/operations.txt` and every schema they
reference, transitively. For the initial 70 operations that is 53 paths and 219 schemas
(`api/apple/openapi.pruned.json`, 0.9 MB).

## Criteria

1. Generates and compiles; `go vet` clean.
2. Enums are typed (`ScreenshotDisplayType`, `Platform`, `AppStoreVersionState`, `BuildUploadState`).
3. Bracketed query params work (`filter[app]`, `filter[preReleaseVersion.version]`, `fields[builds]`).
4. Update requests omit unset attributes: a PATCH that sets only `whatsNew` serializes exactly
   `{"data":{"type":"appStoreVersionLocalizations","id":"X","attributes":{"whatsNew":"..."}}}`.
5. Decodes the fixtures in `internal/apple/asc/testdata`, including heterogeneous `included`.
6. Generated code under ~30k lines, `go build` of the package under 30 s.

## Candidates and results

### 1. oapi-codegen v2.8.0 (`github.com/oapi-codegen/oapi-codegen/v2`)

Config: `generate: {models: true, client: true}`, tried with and without
`generate-types-for-anonymous-schemas` and `resolve-type-name-collisions`.

| # | Result |
|---|--------|
| 1 | **Fail.** Aborts with `duplicate typename 'AppAssetLibraryPlacementType' detected, can't auto-rename` (the inline `type` enum of `AppAssetLibraryPlacement` collides with the schema `AppAssetLibraryPlacementType`; the same for `BuildBundleType`). `resolve-type-name-collisions` does not help. After patching both schemas with `x-go-name` it generates, but the output does not compile: `KidsAgeBand redeclared in this block` (enum constant vs. type) and duplicate methods `AsAppAssetLibraryImageCommonAttributes` (a `oneOf` lists the same `$ref` twice). |
| 2 | Pass in the output (`type Platform string` etc.), plus one enum type per operation and `fields[...]` parameter. |
| 3 | Not testable (does not compile). The output has `form:"filter[app]"` tags. |
| 4 | Not testable. The output uses `*string` with `omitempty`, so it would likely pass. |
| 5 | Not testable. `included` items become union types with `As<Type>()` methods. |
| 6 | **Fail.** 55,836 lines (58,319 with named anonymous types), plus the `oapi-codegen/runtime` dependency. |

### 2. ogen v1.24.0 (`github.com/ogen-go/ogen`)

Config: client only (`paths/server`, `webhooks/server`, `ogen/otel` disabled).

| # | Result |
|---|--------|
| 1 | **Fail.** Aborts with `type name conflict: "AppAssetLibraryPlacementType"` (and `BuildBundleType`). After patching both schemas with `x-ogen-name` it aborts with `Feature "sum types with same names" is not implemented yet`. With `ignore_not_implemented: ["sum types with same names"]` it generates and compiles, but skips `reviewSubmissionItems_createInstance`, which the CLI needs. |
| 2 | Pass (`type Platform string` etc.). |
| 3 | Not tested further; the client encodes `filter[app]` by name. |
| 4 | Not tested further; `OptNilString` fields are omitted when unset. |
| 5 | Not tested further; `included` becomes a sum type per response. |
| 6 | **Fail.** 324,632 lines in 11 files (149,092 of them JSON encoders); `go build` 9.9 s with a cold cache. |

### 3. Own generator `tools/ascgen` (chosen)

Go `text/template` over the pruned spec: 626 lines of Go plus 226 lines of templates.

| # | Result |
|---|--------|
| 1 | Pass. Compiles, `go vet` clean. The generator fails loudly on any identifier collision instead of renaming. |
| 2 | Pass. Every enum (named schema or inline) is a typed string with constants. |
| 3 | Pass. `TestQueryParamsEncoded`. |
| 4 | Pass. `TestPatchOmitsUnsetAttributes` (exact byte comparison of the request body). |
| 5 | Pass. `TestDecodeFixtures`. |
| 6 | Pass. 8,054 lines in 3 files; `go build` of the package about 0.2 s (3.8 s with a cold cache including `net/http`). No dependencies outside the standard library. |

## Decision

Use the own generator `tools/ascgen`. Neither third-party generator handles this spec without
patching it, and both produce far more code than the criteria allow. The own generator only
needs the subset of OpenAPI this spec uses and rejects anything else with an error, so a new spec
version that uses new constructs fails at `go generate` time, not at run time.

## How it works

```
api/apple/operations.txt  + api/apple/openapi.oas.json
  -> tools/ascprune -> api/apple/openapi.pruned.json
  -> tools/ascgen   -> internal/apple/asc/{client,types,operations}.gen.go
```

Both steps run from `internal/apple/asc/generate.go` via `go generate ./...`. `ascprune` keeps
the spec's key order (property order becomes struct field order and so JSON field order),
`ascgen` sorts schemas and operations by name, so the output is byte-identical for the same input;
CI runs `go generate ./... && git diff --exit-code`.

- **Add an operation:** add its `operationId` to `api/apple/operations.txt`, run
  `go generate ./...`, commit the regenerated files. Unknown IDs fail the prune step.
- **Update the spec:** replace `api/apple/openapi.oas.json`, run `go generate ./...`, fix compile
  errors in the callers.

Generated files: `client.gen.go` (client, errors, pagination, `IncludedResource`),
`types.gen.go` (schemas and enums), `operations.gen.go` (one method and params struct per
operation). Never edit them.

## Mapping rules

- **Client.** `asc.NewClient(baseURL, doer)`. `doer` is any `asc.Doer` (`*http.Client`
  implements it); auth, retries and logging are the caller's (B2: `httpx` client plus the JWT).
  `asc.DefaultBaseURL` is `https://api.appstoreconnect.apple.com`; tests pass the `httptest` URL.
- **Methods.** `operationId` in PascalCase: `builds_getCollection` -> `BuildsGetCollection`.
  Signature: `(ctx[, params XParams][, body XRequest]) (*XResponse, error)`, or just `error` for
  204 responses. `params` exists if the operation has any parameter; `body` if it has a request body.
- **Params structs** (`<Method>Params`): path `id` -> `ID string`; `filter[preReleaseVersion.version]`
  -> `FilterPreReleaseVersionVersion []string`; `fields[builds]` -> `FieldsBuilds`, `include` ->
  `Include`, `sort` -> `Sort` (all `[]string`, sent comma-joined, empty = not sent); `limit` and
  `limit[x]` -> `int` (0 = not sent); `exists[x]` -> `*bool`. The field comment names the original
  parameter. Enum-valued query params are plain strings: `string(asc.PlatformIos)`.
- **Schemas.** A component schema keeps its name (`BuildsResponse`, `HttpHeader`). An inline
  object or enum is named after its owner plus the property in PascalCase:
  `Build.attributes` -> `BuildAttributes`, its `processingState` -> `BuildAttributesProcessingState`,
  `AppStoreVersionLocalizationUpdateRequest.data.attributes` ->
  `AppStoreVersionLocalizationUpdateRequestDataAttributes`; array items get `Item`.
  The `// X is generated from ...` comment of each type names its origin.
- **Field names.** PascalCase of the JSON name, with the words `Id` and `Url` as `ID` and `URL`
  (`supportUrl` -> `SupportURL`).
- **Optional fields.** Not in `required` (or `nullable`): pointer with `omitempty` (slices and maps
  without pointer). Set them with Go's `new(value)`: `WhatsNew: new("text")`,
  `Platform: new(asc.PlatformIos)`. Unset fields are not sent; there is no way to send JSON `null`.
  Read them with a nil check: the API omits attributes not listed in `fields[...]`.
- **Enums.** Typed strings; constants are type name plus PascalCase value:
  `ScreenshotDisplayTypeAppIphone67`, `AppStoreVersionStatePrepareForSubmission`, `PlatformIos`.
- **`ResourceType`.** Every single-valued `type` property shares one enum:
  `asc.ResourceTypeBuilds`, `asc.ResourceTypeAppStoreVersionLocalizations`, ...
- **`ResourceIdentifier`.** Every inline `{type, id}` object (relationship data, linkage request
  items) is `asc.ResourceIdentifier{Type, ID}`.
- **`included`.** A heterogeneous `included` array is `[]asc.IncludedResource` (`Type`, `ID`,
  `Raw`); `Decode` it into the type matching `Type`. Responses whose `included` can only hold one
  type keep that type (`AppScreenshotResponse.Included []AppScreenshotSet`).
- **Other `oneOf`/`allOf`.** Object variants are merged into one struct (all `oneOf` fields
  optional), e.g. `ErrorResponseErrorsItemSource{Pointer, Parameter}`; anything else is
  `json.RawMessage`.
- **Formats.** `date-time` -> `time.Time`, `int64` -> `int64`, `integer` -> `int`,
  `number` -> `float64`; other string formats (`uri`, `email`) stay `string`.
- **Errors.** Every non-2xx response returns `*asc.ResponseError{StatusCode, Response}` where
  `Response` is the decoded `ErrorResponse` (empty if the body was not JSON). Mapping to
  `clierr.Error` is B2's job.
- **Pagination.** `asc.GetNext[T](ctx, c, next)` fetches a `links.next` URL into `*T`. It refuses
  URLs outside the client's base URL, so the token never leaves for another host.

## Cheat sheet

All snippets assume `c := asc.NewClient(baseURL, httpClient)` and `ctx`.

List with filters:

```go
resp, err := c.BuildsGetCollection(ctx, asc.BuildsGetCollectionParams{
	FilterApp:                      []string{appID},
	FilterPreReleaseVersionVersion: []string{"2.3.0"},
	FilterProcessingState:          []string{string(asc.BuildAttributesProcessingStateValid)},
	FieldsBuilds:                   []string{"version", "uploadedDate", "processingState"},
	Sort:                           []string{"-uploadedDate"},
	Limit:                          200,
})
if err != nil {
	return err
}
for _, b := range resp.Data {
	if b.Attributes != nil && b.Attributes.Version != nil {
		fmt.Println(b.ID, *b.Attributes.Version)
	}
}
```

Follow `links.next`:

```go
page, err := c.BuildsGetCollection(ctx, asc.BuildsGetCollectionParams{FilterApp: []string{appID}, Limit: 200})
if err != nil {
	return err
}
builds := page.Data
for page.Links.Next != nil {
	if page, err = asc.GetNext[asc.BuildsResponse](ctx, c, *page.Links.Next); err != nil {
		return err
	}
	builds = append(builds, page.Data...)
}
```

Get one:

```go
v, err := c.AppStoreVersionsGetInstance(ctx, asc.AppStoreVersionsGetInstanceParams{
	ID:      versionID,
	Include: []string{"build"},
})
if err != nil {
	return err
}
if s := v.Data.Attributes.AppVersionState; s != nil && *s == asc.AppVersionStatePrepareForSubmission {
	// editable
}
```

Create with relationships:

```go
loc, err := c.AppStoreVersionLocalizationsCreateInstance(ctx, asc.AppStoreVersionLocalizationCreateRequest{
	Data: asc.AppStoreVersionLocalizationCreateRequestData{
		Type: asc.ResourceTypeAppStoreVersionLocalizations,
		Attributes: asc.AppStoreVersionLocalizationCreateRequestDataAttributes{
			Locale:   "de-DE",
			WhatsNew: new("Fehlerbehebungen"),
		},
		Relationships: asc.AppStoreVersionLocalizationCreateRequestDataRelationships{
			AppStoreVersion: asc.AppStoreVersionLocalizationCreateRequestDataRelationshipsAppStoreVersion{
				Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersions, ID: versionID},
			},
		},
	},
})
if err != nil {
	return err
}
locID := loc.Data.ID
```

Partial update (only non-nil attributes are sent):

```go
_, err := c.AppStoreVersionLocalizationsUpdateInstance(ctx,
	asc.AppStoreVersionLocalizationsUpdateInstanceParams{ID: locID},
	asc.AppStoreVersionLocalizationUpdateRequest{Data: asc.AppStoreVersionLocalizationUpdateRequestData{
		Type: asc.ResourceTypeAppStoreVersionLocalizations,
		ID:   locID,
		Attributes: &asc.AppStoreVersionLocalizationUpdateRequestDataAttributes{
			WhatsNew: new("Bug fixes"),
		},
	}})
```

Set a to-one relationship (attach a build):

```go
err := c.AppStoreVersionsBuildUpdateToOneRelationship(ctx,
	asc.AppStoreVersionsBuildUpdateToOneRelationshipParams{ID: versionID},
	asc.AppStoreVersionBuildLinkageRequest{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeBuilds, ID: buildID}})
```

Delete:

```go
err := c.AppScreenshotsDeleteInstance(ctx, asc.AppScreenshotsDeleteInstanceParams{ID: screenshotID})
```

To-many relationship add and remove (replace works the same with
`AppScreenshotSetsAppScreenshotsReplaceToManyRelationship`):

```go
testers := asc.BetaGroupBetaTestersLinkagesRequest{Data: []asc.ResourceIdentifier{
	{Type: asc.ResourceTypeBetaTesters, ID: testerID},
}}
params := asc.BetaGroupsBetaTestersCreateToManyRelationshipParams{ID: groupID}
if err := c.BetaGroupsBetaTestersCreateToManyRelationship(ctx, params, testers); err != nil {
	return err
}
err := c.BetaGroupsBetaTestersDeleteToManyRelationship(ctx,
	asc.BetaGroupsBetaTestersDeleteToManyRelationshipParams{ID: groupID}, testers)
```

Read `included`:

```go
resp, err := c.ReviewSubmissionsGetCollection(ctx, asc.ReviewSubmissionsGetCollectionParams{
	FilterApp: []string{appID},
	Include:   []string{"items", "appStoreVersionForReview"},
})
if err != nil {
	return err
}
versions := map[string]asc.AppStoreVersion{}
for _, inc := range resp.Included {
	if inc.Type != asc.ResourceTypeAppStoreVersions {
		continue
	}
	var v asc.AppStoreVersion
	if err := inc.Decode(&v); err != nil {
		return err
	}
	versions[v.ID] = v
}
for _, s := range resp.Data {
	if r := s.Relationships; r != nil && r.AppStoreVersionForReview != nil && r.AppStoreVersionForReview.Data != nil {
		forReview := versions[r.AppStoreVersionForReview.Data.ID]
		fmt.Println(s.ID, forReview.ID)
	}
}
```

Decode an error response:

```go
var re *asc.ResponseError
if errors.As(err, &re) {
	// re.StatusCode: 409
	for _, e := range re.Response.Errors {
		// e.Status "409", e.Code "ENTITY_ERROR.ATTRIBUTE.INVALID", e.Title, e.Detail
		if e.Source != nil && e.Source.Pointer != nil {
			// *e.Source.Pointer: "/data/attributes/whatsNew"
		}
	}
}
```

## Consequences

- Adding an operation is one line in `operations.txt` plus `go generate`. A construct the generator
  does not know (a new parameter style, an inline request body, a non-JSON response) stops
  generation with an error naming the operation or schema; extend `tools/ascgen` then.
- Names are long but mechanical; derive them from the spec path instead of guessing, or search
  `types.gen.go` for the `generated from` comment.
- JSON `null` cannot be sent. Clearing a text field means sending `""` where the API accepts it.
- Query enum values are not checked at compile time.
