# Roomzin Go SDK

Official Go SDK for [Roomzin](https://m-javani.github.io/roomzin-doc/) — a high-performance in-memory inventory engine for booking platforms.

The SDK provides a robust, idiomatic Go interface for communicating with Roomzin servers in both standalone and clustered deployments. It automatically manages routing, failover, connection pooling, and cluster topology changes.

---

## Features

- Automatic request routing (leader for writes, followers for reads)
- Built-in failover and cluster discovery
- Connection pooling
- Standalone and clustered deployment support
- Type-safe API with context support
- Reusable, concurrency-safe client

---

## Requirements

- Go 1.21 or later
- Roomzin Server v1.x

---

## Installation

```bash
go get github.com/roomzin/roomzin-go
```

---

## Client Setup

### Standalone

```go
import "github.com/roomzin/roomzin-go/single"

cfg, err := single.NewConfigBuilder().
    WithHost("127.0.0.1").
    WithTCPPort(7777).
    WithToken("abc123").
    WithTimeout(5 * time.Second).
    WithKeepAlive(30 * time.Second).
    Build()

client, err := single.New(&cfg)
defer client.Close()
```

### Cluster (Static Discovery)

```go
import "github.com/roomzin/roomzin-go/cluster"

staticDiscovery := []types.NodeAddr{
    {NodeID: "roomzin-0", Host: "172.20.0.10", TcpPort: 7777, ApiPort: 8080},
    {NodeID: "roomzin-1", Host: "172.20.0.11", TcpPort: 7777, ApiPort: 8080},
}

cfg, err := cluster.NewConfigBuilder().
    WithSeedNodeIDs("roomzin-0,roomzin-1").
    WithStaticDiscovery(staticDiscovery).
    WithTCPPort(7777).
    WithAPIPort(8080).
    WithToken("abc123").
    WithTimeout(30 * time.Second).
    WithKeepAlive(30 * time.Second).
    Build()

client, err := cluster.New(&cfg)
defer client.Close()
```

### Cluster (HTTP Discovery)

```go
cfg, err := cluster.NewConfigBuilder().
    WithSeedNodeIDs("roomzin-0,roomzin-1,roomzin-2").
    WithHTTPDiscovery("http://discovery-service:8080/nodes").
    WithTCPPort(7777).
    WithAPIPort(8080).
    WithToken("abc123").
    WithTimeout(30 * time.Second).
    WithKeepAlive(30 * time.Second).
    Build()
```

---

## Discovery Configuration

Roomzin SDKs need to know how to reach each Roomzin node in the cluster. The cluster nodes communicate with each other using internal address resolvers, but the SDK as an external client needs actual network addresses (IP:port or hostname:port) to connect.

The SDK fetches the cluster topology from the Roomzin cluster itself. This topology includes the node identities of the leader and followers. The SDK then uses discovery to resolve these node identities into actual network addresses.

Two discovery modes are supported:

### Static Discovery

The SDK gets the mapping once in config and never updates it. Use this when your cluster nodes have stable, predictable addresses.

### HTTP Discovery

The SDK periodically fetches the mapping from an HTTP endpoint. Use this when cluster nodes are dynamic (e.g., Kubernetes pods with changing IPs).

---

## Property Management

### SetProp
Adds or updates a property.

```go
err := client.SetProp(ctx, types.SetPropPayload{
    Segment:      "downtown",
    Area:         "manhattan",
    PropertyID:   "hotel_123",
    PropertyType: "hotel",
    Category:     "luxury",
    Stars:        4,
    Latitude:     40.7128,
    Longitude:    -74.0060,
    Amenities:    []string{"wifi", "pool", "gym"},
})
```

### SearchProp
Searches properties by segment, area, type, or location.

```go
// By segment
ids, err := client.SearchProp(ctx, types.SearchPropPayload{
    Segment: "downtown",
})

// By area
ids, err := client.SearchProp(ctx, types.SearchPropPayload{
    Segment: "downtown",
    Area:    "manhattan",
})

// By location (radius search)
lat := 40.7128
lon := -74.0060
ids, err := client.SearchProp(ctx, types.SearchPropPayload{
    Segment:   "downtown",
    Latitude:  &lat,
    Longitude: &lon,
})
```

### PropExist
Checks if a property exists.

```go
exists, err := client.PropExist(ctx, "hotel_123")
```

### PropRoomExist
Checks if a specific room type exists for a property.

```go
exists, err := client.PropRoomExist(ctx, types.PropRoomExistPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
})
```

### PropRoomList
Lists all room types for a property.

```go
rooms, err := client.PropRoomList(ctx, "hotel_123")
```

### PropRoomDateList
Lists dates with availability data for a property and room type.

```go
dates, err := client.PropRoomDateList(ctx, types.PropRoomDateListPayload{
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
err := client.SetRoomPkg(ctx, types.SetRoomPkgPayload{
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
newAvail, err := client.SetRoomAvl(ctx, types.UpdRoomAvlPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
    Amount:     20,
})
```

### IncRoomAvl
Increases availability (e.g., on cancellation).

```go
newAvail, err := client.IncRoomAvl(ctx, types.UpdRoomAvlPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
    Amount:     1,
})
```

### DecRoomAvl
Decreases availability (e.g., on booking).

```go
newAvail, err := client.DecRoomAvl(ctx, types.UpdRoomAvlPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
    Amount:     2,
})
```

### GetPropRoomDay
Gets availability and pricing for a specific room on a specific date.

```go
day, err := client.GetPropRoomDay(ctx, types.GetRoomDayRequest{
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

results, err := client.SearchAvail(ctx, types.SearchAvailPayload{
    Segment:     "downtown",
    RoomType:    "suite",
    Date:        []string{"2026-07-20", "2026-07-21"},
    Limit:       &limit,
    MinPrice:    &minPrice,
    MaxPrice:    &maxPrice,
    Amenities:   []string{"wifi", "pool"},
    RateFeature: []string{"free_cancellation"},
})

for _, result := range results {
    fmt.Printf("Property: %s\n", result.PropertyID)
    for _, day := range result.Days {
        fmt.Printf("  %s: Avail %d, Price %d\n", day.Date, day.Availability, day.FinalPrice)
    }
}
```

### GetSegments
Lists all active segments with their property counts.

```go
segments, err := client.GetSegments(ctx)
for _, seg := range segments {
    fmt.Printf("%s: %d properties\n", seg.Segment, seg.Count)
}
```

### GetCodecs
Gets the current codec registry (used internally for validation).

```go
codecs, err := client.GetCodecs(ctx)
fmt.Println(codecs.RateFeatures)
```

---

## Delete Operations

### DelRoomDay
Deletes availability for a specific room on a specific date.

```go
err := client.DelRoomDay(ctx, types.DelRoomDayRequest{
    PropertyID: "hotel_123",
    RoomType:   "suite",
    Date:       "2026-07-20",
})
```

### DelPropDay
Deletes all data for a property on a specific date.

```go
err := client.DelPropDay(ctx, types.DelPropDayPayload{
    PropertyID: "hotel_123",
    Date:       "2026-07-20",
})
```

### DelPropRoom
Deletes a room type from a property.

```go
err := client.DelPropRoom(ctx, types.DelPropRoomPayload{
    PropertyID: "hotel_123",
    RoomType:   "suite",
})
```

### DelProp
Deletes an entire property.

```go
err := client.DelProp(ctx, "hotel_123")
```

### DelSegment
Deletes a segment and all properties within it.

```go
err := client.DelSegment(ctx, "downtown")
```

---

## Error Handling

All methods return `*types.RoomzinError`. Use the helper functions to classify errors:

```go
if err := client.SetRoomPkg(ctx, payload); err != nil {
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
| **Retry** | Temporary server condition (429, 503, 308) | Retry with backoff |
| **Cluster** | Topology change or node failure | Client auto-handles |
| **Internal** | Unexpected server response | Log and investigate |

---

## Client Lifecycle

Create a **single client** during application startup and reuse it throughout your application.

```go
// ✅ Good - create once, reuse
client, err := single.New(&cfg)
defer client.Close()
// Use client everywhere

// ❌ Bad - creating per request
for _, req := range requests {
    client, _ := single.New(&cfg)  // Don't do this
    client.SetRoomPkg(ctx, req)
    client.Close()
}
```

The client is safe for concurrent use and manages TCP connections internally.

---

## API Reference

For the complete interface definition, see [`api/client.go`](api/client.go). All types are documented with GoDoc comments.

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
- **Issues**: [GitHub Issues](https://github.com/roomzin/roomzin-go/issues)
- **Security**: [mehdy.javany@gmail.com](mailto:mehdy.javany@gmail.com)

---

## Related Repositories

- [Roomzin Quickstart](https://github.com/m-javani/roomzin-quickstart) — Local Docker cluster
- [Roomzin Bench](https://github.com/m-javani/roomzin-bench) — Benchmarking tool