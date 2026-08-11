# Roomzin Go SDK

Official Go SDK for [Roomzin](https://m-javani.github.io/roomzin-doc/) — a high-performance in-memory inventory engine for booking platforms.

The SDK provides a robust, idiomatic Go interface for communicating with Roomzin servers in both standalone and clustered deployments. It automatically handles connection management, request/response demuxing, and self-healing reconnections.

---

## Features

- Unified client for standalone and cluster deployments
- Automatic request routing (writes to leader, reads to followers) via router
- Built-in connection self-healing
- Type-safe API with context and segment support
- Reusable, concurrency-safe client
- Minimal allocation hot path

---

## Requirements

- Go 1.21 or later
- Roomzin Server v1.x
- Roomzin Router (for cluster mode)

---

## Installation

```bash
go get github.com/m-javani/roomzin-go
```

---

## Client Setup

The SDK provides a single unified client that works in both standalone and cluster modes.

### Standalone Mode

Connect directly to a standalone Roomzin server:

```go
import "github.com/m-javani/roomzin-go/pkg/client"

cfg, err := client.NewConfigBuilder().
    WithAddr("127.0.0.1").
    WithPort(7777).
    WithTimeout(5 * time.Second).
    WithKeepAlive(30 * time.Second).
    WithMode(client.StandaloneMode).
    Build()
if err != nil {
    log.Fatal(err)
}

cli, err := client.New(&cfg)
if err != nil {
    log.Fatal(err)
}
defer cli.Close()
```

### Cluster Mode (via Router)

Connect to a Roomzin cluster through the router:

```go
import "github.com/m-javani/roomzin-go/pkg/client"

cfg, err := client.NewConfigBuilder().
    WithAddr("router.example.com").
    WithPort(7777).
    WithTimeout(30 * time.Second).
    WithKeepAlive(30 * time.Second).
    WithMode(client.ClusterMode).
    Build()
if err != nil {
    log.Fatal(err)
}

cli, err := client.New(&cfg)
if err != nil {
    log.Fatal(err)
}
defer cli.Close()
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithAddr()` | Server or router address | Required |
| `WithPort()` | TCP port | Required |
| `WithTimeout()` | Request timeout | 2s |
| `WithKeepAlive()` | TCP keep-alive interval | 30s |
| `WithMode()` | `StandaloneMode` or `ClusterMode` | `StandaloneMode` |

---

## Segment Routing

In cluster mode, every request must specify a segment. The router uses this to route the request to the correct shard.

```go
ctx := context.Background()
segment := "us-east"

// All API methods accept segment as a parameter
err := cli.SetProp(ctx, segment, payload)
```

In standalone mode, the segment parameter is ignored but still required for API compatibility. This allows you to switch between standalone and cluster modes without changing your business logic.

---

## Property Management

### SetProp
Adds or updates a property.

```go
err := cli.SetProp(ctx, "downtown", types.SetPropPayload{
    Segment:      "downtown",
    Area:         "manhattan",
    PropertyID:   "hotel_123",
    PropertyType: "hotel",
    Category:     "luxury",
    Stars:        4,
    Latitude:     40.7128,
    Longitude:    -74.0060,
    Amenities:    []uint32{1, 2, 3}, // amenity codes
})
```

### SearchProp
Searches properties by segment, area, type, or location.

```go
// By segment
ids, err := cli.SearchProp(ctx, "downtown", types.SearchPropPayload{
    Segment: "downtown",
})

// By area
ids, err := cli.SearchProp(ctx, "downtown", types.SearchPropPayload{
    Segment: "downtown",
    Area:    "manhattan",
})

// By location (radius search)
lat := 40.7128
lon := -74.0060
ids, err := cli.SearchProp(ctx, "downtown", types.SearchPropPayload{
    Segment:   "downtown",
    Latitude:  &lat,
    Longitude: &lon,
})
```

### PropExist
Checks if a property exists.

```go
exists, err := cli.PropExist(ctx, "downtown", "hotel_123")
```

### PropRoomExist
Checks if a specific room type exists for a property.

```go
exists, err := cli.PropRoomExist(ctx, "downtown", types.PropRoomExistPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
})
```

### PropRoomList
Lists all room types for a property.

```go
rooms, err := cli.PropRoomList(ctx, "downtown", "hotel_123")
```

### PropRoomDateList
Lists dates with availability data for a property and room type.

```go
dates, err := cli.PropRoomDateList(ctx, "downtown", types.PropRoomDateListPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
})
```

---

## Room Package Management

### SetRoomPkg
Sets availability, price, and rate features for a room type on a date.

```go
avail := uint8(10)
price := uint32(199)
err := cli.SetRoomPkg(ctx, "downtown", types.SetRoomPkgPayload{
    PropertyID:   "hotel_123",
    RoomType:     "suite",
    Date:         "2026-07-20",
    Availability: &avail,
    FinalPrice:   &price,
    RateFeature:  []string{"free_cancellation", "breakfast_included"},
})
```

### SetRoomAvl
Sets exact availability for a room type on a specific date.

```go
newAvail, err := cli.SetRoomAvl(ctx, "downtown", types.UpdRoomAvlPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
    Amount:     20,
})
```

### IncRoomAvl
Increases availability (e.g., on cancellation).

```go
newAvail, err := cli.IncRoomAvl(ctx, "downtown", types.UpdRoomAvlPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
    Amount:     1,
})
```

### DecRoomAvl
Decreases availability (e.g., on booking).

```go
newAvail, err := cli.DecRoomAvl(ctx, "downtown", types.UpdRoomAvlPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
    Amount:     2,
})
```

### GetPropRoomDay
Gets availability and pricing for a specific room on a specific date.

```go
day, err := cli.GetPropRoomDay(ctx, "downtown", types.GetRoomDayRequest{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
})
fmt.Printf("Avail: %d, Price: %d\n", day.Availability, day.FinalPrice)
```

---

## Search & Query

### SearchAvail
Searches available rooms by filters.

```go
limit := uint64(50)
minPrice := uint32(100)
maxPrice := uint32(300)

results, err := cli.SearchAvail(ctx, "downtown", types.SearchAvailPayload{
    Segment:     "downtown",
    RoomType:    "suite",
    Date:        []string{"2026-07-20", "2026-07-21"},
    Limit:       &limit,
    MinPrice:    &minPrice,
    MaxPrice:    &maxPrice,
    Amenities:   []uint32{1, 2},
    RateFeature: []string{"free_cancellation"},
})

for _, result := range results {
    fmt.Printf("Property: %s\n", result.PropertyID)
    for _, day := range result.Days {
        fmt.Printf("  %s: Avail %d, Price %d\n", day.Date, day.Availability, day.FinalPrice)
    }
}
```

---

## Delete Operations

### DelRoomDay
Deletes availability for a specific room on a specific date.

```go
err := cli.DelRoomDay(ctx, "downtown", types.DelRoomDayRequest{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
})
```

### DelPropDay
Deletes all data for a property on a specific date.

```go
err := cli.DelPropDay(ctx, "downtown", types.DelPropDayRequest{
    PropertyID: "hotel_123",
    Date:       "2026-07-20",
})
```

### DelPropRoom
Deletes a room type from a property.

```go
err := cli.DelPropRoom(ctx, "downtown", types.DelPropRoomPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
})
```

### DelProp
Deletes an entire property.

```go
err := cli.DelProp(ctx, "downtown", "hotel_123")
```

### DelSegment
Deletes a segment and all properties within it.

```go
err := cli.DelSegment(ctx, "downtown")
```

---

## Error Handling

All methods return errors. Use the helper functions to classify errors:

```go
if err := cli.SetRoomPkg(ctx, segment, payload); err != nil {
    var rz *types.RoomzinError
    if errors.As(err, &rz) {
        switch {
        case types.IsRequest(err):
            // Business rule violation - fix the request
            log.Printf("Request error: %s", rz.Message)
        case types.IsRetry(err):
            // Temporary condition - retry with backoff
            time.Sleep(100 * time.Millisecond)
            // retry...
        case types.IsCluster(err):
            // Topology change - client auto-handles
            log.Printf("Cluster error: %s", rz.Message)
        default:
            // Fatal error
            return fmt.Errorf("fatal: %w", err)
        }
    }
}
```

### Error Categories

| Category | Description | Action |
|----------|-------------|--------|
| **Request** | Invalid input or business rule violation | Fix request, don't retry |
| **Retry** | Temporary server condition (429, 503) | Retry with backoff |
| **Cluster** | Topology change or node failure | Client auto-handles |
| **Internal** | Unexpected server response | Log and investigate |

---

## Architecture

### Standalone Mode

```
[SDK] → [Standalone Server]
```

- Single TCP connection
- Direct communication
- Self-healing on disconnection

### Cluster Mode

```
[SDK] → [Router] → [Shard Leader/Followers]
```

- SDK sends segment and isWrite flag in header
- Router routes writes to leader, reads to followers
- Router handles cluster topology
- SDK maintains single connection to router

### Protocol

The SDK uses a framed binary protocol:

**Standalone Frame:**
```
[0xFF][ClrID(4)][TotalLen(4)][Payload]
```

**Router Frame:**
```
[0xFE][TotalLen(4)][SegmentLen(1)][Segment(n)][IsWrite(1)][ShardFrame]
```

Where `ShardFrame` is the standalone frame format.

---

## Client Lifecycle

Create a **single client** during application startup and reuse it throughout your application.

```go
// ✅ Good - create once, reuse
cli, err := client.New(&cfg)
defer cli.Close()
// Use cli everywhere

// ❌ Bad - creating per request
for _, req := range requests {
    cli, _ := client.New(&cfg)  // Don't do this
    cli.SetRoomPkg(ctx, segment, req)
    cli.Close()
}
```

The client is safe for concurrent use and manages TCP connections internally.

---

## Examples

A complete smoke example is available in the `examples/` directory. It demonstrates the SDK's core features and can be run as a reference implementation or to verify your Roomzin setup.

```bash
cd examples
go run smoke.go
```

---

## Documentation

For Roomzin concepts, deployment, and administration:

[https://m-javani.github.io/roomzin-doc/docs.html](https://m-javani.github.io/roomzin-doc/docs.html)

---

## Contributing

Contributions are welcome! Please open an issue before proposing large changes.

All contributions are subject to the BUSL-1.1 License terms.

---

## License

This SDK is licensed under the [BUSL-1.1 License](LICENSE).

**Note:** This SDK communicates with Roomzin Server, which requires a valid Roomzin license.

---

## Support

- **Documentation**: [roomzin-doc](https://m-javani.github.io/roomzin-doc/)
- **Community Q&A**: [GitHub Discussions](https://github.com/m-javani/roomzin-doc/discussions)
- **Issues**: [GitHub Issues](https://github.com/m-javani/roomzin-go/issues)
- **Security**: [mehdy.javany@gmail.com](mailto:mehdy.javany@gmail.com)

---

## Related Repositories

- [Roomzin Quickstart](https://github.com/m-javani/roomzin-quickstart) — Local Docker cluster
- [Roomzin Bench](https://github.com/m-javani/roomzin-bench) — Benchmarking tool
