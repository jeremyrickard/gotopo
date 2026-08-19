# gotopo

`gotopo` is an unofficial Go client for the undocumented CalTopo API. It is a
Go reimplementation of [`caltopo_python` 2.x](https://github.com/ncssar/caltopo_python)
with context-aware synchronous methods, typed operation options, an extensible
feature model, a concurrency-safe map cache, and pure-Go geometry operations.

> [!WARNING]
> This project is not maintained or supported by CalTopo. Mutating calls can
> permanently edit or delete map data, and CalTopo may not provide undo. Export
> a full GeoJSON backup before using write operations.

## Install

```sh
go get github.com/jeremyrickard/gotopo
```

Go 1.23 or newer is required.

## Credentials

Hosted `caltopo.com` requests require a credential ID, public key, and account
ID. Follow the upstream
[credential instructions](https://caltopo-python.readthedocs.io/en/latest/credentials.html)
to create them. Treat these values as secrets.

```go
credentials := gotopo.Credentials{
    ID:        os.Getenv("CALTOPO_CREDENTIAL_ID"),
    Key:       os.Getenv("CALTOPO_KEY"),
    AccountID: os.Getenv("CALTOPO_ACCOUNT_ID"),
}

client, err := gotopo.NewClient(
    gotopo.WithEndpoint("caltopo.com"),
    gotopo.WithCredentials(credentials),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

Existing `caltopo_python` INI files are also supported:

```go
credentials, err := gotopo.CredentialsFromINI("caltopo.ini", "user@example.com")
```

For CalTopo Desktop, use its local address. Credentials are not required for
ordinary local map operations:

```go
client, err := gotopo.NewClient(gotopo.WithEndpoint("localhost:8080"))
```

## Open a map and query features

Opening a map performs an initial blocking sync, then keeps the cache updated
in the background until `CloseMap`, `StopSync`, or `Close` is called. Set
`DisableBackgroundSync` when manual refreshes are preferred.

```go
ctx := context.Background()
err = client.OpenMap(ctx, "ABCDE", gotopo.OpenMapOptions{
    DisableBackgroundSync: false,
})
if err != nil {
    log.Fatal(err)
}

assignments, err := client.GetFeatures(ctx, gotopo.FeatureFilter{
    Class: "Assignment",
})
```

All returned features are copies. CalTopo properties remain
`map[string]any` because the private API changes over time; common values have
typed helpers such as `feature.Class()`, `feature.Title()`, and
`feature.Properties.String("folderId")`.

## Create, edit, and delete

```go
marker, err := client.AddMarker(ctx, gotopo.MarkerOptions{
    Latitude:  39.1,
    Longitude: -120.2,
    Title:     "Command Post",
    Symbol:    "cp",
})
if err != nil {
    log.Fatal(err)
}

marker, err = client.EditMarkerDescription(ctx, marker.ID, "Primary ICP")
if err != nil {
    log.Fatal(err)
}

_, err = client.DeleteMarker(ctx, marker.ID)
```

Creation methods accept `CreateOptions{Queue: true}` to defer writes. `Flush`
sends the queued objects in one save request. A failed flush retains the queue
for retry.

SAR methods include operational periods, area and line assignments, and live
tracks:

```go
assignment, err := client.AddAreaAssignment(ctx, gotopo.AssignmentOptions{
    Points: []gotopo.Position{
        {-120.2, 39.0}, {-120.1, 39.0}, {-120.1, 39.1}, {-120.2, 39.0},
    },
    Letter:       "A",
    Number:       "1",
    ResourceType: gotopo.ResourceGround,
    Priority:     gotopo.PriorityHigh,
})
```

## Cache events

Handlers run outside internal locks and should return promptly.

```go
client, err := gotopo.NewClient(
    gotopo.WithEventHandlers(gotopo.EventHandlers{
        FeatureAdded: func(feature gotopo.Feature) {
            log.Printf("added %s", feature.ID)
        },
        Disconnected: func(err error) {
            log.Printf("sync disconnected: %v", err)
        },
    }),
)
```

## Geometry

`Cut`, `Expand`, `Crop`, and `GetBounds` use the pure-Go
[`simplefeatures`](https://github.com/peterstace/simplefeatures) geometry
engine. Feature references may contain an ID, title, or complete feature.

```go
result, err := client.Crop(
    ctx,
    gotopo.FeatureRef{ID: "target-id"},
    gotopo.FeatureRef{ID: "boundary-id"},
    gotopo.CropOptions{NoDraw: true},
)
```

Coordinates use GeoJSON order: longitude, latitude. By default, obviously
swapped coordinates are corrected before writes. Configure this with
`WithPointValidation`.

## `caltopo_python` compatibility

| Python 2.x method | Go equivalent |
| --- | --- |
| `CaltopoSession(...)` | `NewClient(...)` |
| `openMap`, `closeMap` | `OpenMap`, `CreateMap`, `CloseMap` |
| `getAccountData` | `GetAccountData` |
| `getMapList`, `getAllMapLists` | `GetMapLists`, `GetAllMapLists` |
| `getMapTitle` | `GetMapTitle` |
| `getAccountsAndFolders`, `getGroupAccountTitles` | `GetAccountsAndFolders`, `GetGroupAccountTitles` |
| `addFolder`, `addMarker`, `addLine`, `addPolygon` | `AddFolder`, `AddMarker`, `AddLine`, `AddPolygon` |
| `addOperationalPeriod` | `AddOperationalPeriod` |
| `addAssignment`, `addAreaAssignment`, `addLineAssignment` | `AddAreaAssignment`, `AddLineAssignment` |
| `addLiveTrack`, `updateLiveTrack`, `stopLiveTrack` | `AddLiveTrack`, `UpdateLiveTrack`, `StopLiveTrack` |
| `flush` | `Flush` |
| `getFeature`, `getFeatures` | `GetFeature`, `GetFeatures` |
| `editFeature`, `moveMarker`, `editMarkerDescription` | `EditFeature`, `MoveMarker`, `EditMarkerDescription` |
| `delFeature`, `delFeatures`, `delMarker`, `delMarkers` | `DeleteFeature`, `DeleteFeatures`, `DeleteMarker` |
| `cut`, `expand`, `crop`, `getBounds` | `Cut`, `Expand`, `Crop`, `GetBounds` |

Python's callback request queue is intentionally replaced by normal Go
concurrency: every network operation accepts `context.Context`, returns a typed
result and `error`, and can be called from a goroutine. Background map sync is
retained as an explicit cache lifecycle feature.

Private Python helpers, debug dump files, and Fiddler-specific configuration
are not exported APIs.

## License

GPL-3.0. See [LICENSE](LICENSE).
