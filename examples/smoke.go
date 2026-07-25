package main

import (
	"fmt"
	"log"
	"time"

	"github.com/m-javani/roomzin-go/pkg/api"
	"github.com/m-javani/roomzin-go/pkg/cluster"
	"github.com/m-javani/roomzin-go/pkg/single"
	"github.com/m-javani/roomzin-go/pkg/types"
)

func GetStandaloneClient() (api.CacheClientAPI, error) {
	cfg, err := single.NewConfigBuilder().
		WithHost("127.0.0.1").
		WithTCPPort(7777).
		WithToken("abc123").
		WithTimeout(5 * time.Second).
		WithKeepAlive(30 * time.Second).
		Build()
	if err != nil {
		return nil, err
	}
	return single.New(&cfg)
}

// change the connection config to match your running roomzin cluster
func GetClusterClient() (api.CacheClientAPI, error) {
	staticDiscovery := []types.NodeAddr{
		{
			NodeID:  "roomzin-0",
			Host:    "172.20.0.10",
			TcpPort: 7777,
			ApiPort: 8080,
		},
		{
			NodeID:  "roomzin-1",
			Host:    "172.20.0.11",
			TcpPort: 7777,
			ApiPort: 8080,
		},
		{
			NodeID:  "roomzin-2",
			Host:    "172.20.0.12",
			TcpPort: 7777,
			ApiPort: 8080,
		},
	}
	cfg, err := cluster.NewConfigBuilder().
		WithSeedNodeIDs("roomzin-0,roomzin-1,roomzin-2").
		WithStaticDiscovery(staticDiscovery).
		WithTCPPort(7777).
		WithAPIPort(8080).
		WithToken("abc123").
		WithTimeout(30 * time.Second).
		WithKeepAlive(30 * time.Second).
		Build()
	if err != nil {
		return nil, err
	}

	return cluster.New(&cfg)
}

// ============================================================================
// CONFIGURATION
// ============================================================================

// Change this to "cluster" to test against a Roomzin cluster
const mode = "standalone"

// Standalone configuration
const (
	standaloneHost = "127.0.0.1"
	standalonePort = 7777
	token          = "abc123"
	timeout        = 5 * time.Second
)

// Cluster configuration (update these IPs to match your cluster)
var clusterNodes = []types.NodeAddr{
	{NodeID: "roomzin-0", Host: "172.20.0.10", TcpPort: 7777, ApiPort: 8080},
	{NodeID: "roomzin-1", Host: "172.20.0.11", TcpPort: 7777, ApiPort: 8080},
	{NodeID: "roomzin-2", Host: "172.20.0.12", TcpPort: 7777, ApiPort: 8080},
}

// Test data parameters
const (
	numSegments        = 2
	numPropsPerSegment = 3
	numRoomsPerProp    = 2
	numDays            = 3
)

// ============================================================================
// MAIN FUNCTION - CLEAR LINEAR FLOW
// ============================================================================

func createClient() (api.CacheClientAPI, error) {
	if mode == "standalone" {
		return GetStandaloneClient()
	}

	return GetClusterClient()
}

func main() {
	fmt.Println("=== Roomzin API Example ===")
	fmt.Printf("Mode: %s\n\n", mode)

	// -------------------------------------------------------------------------
	// STEP 1: Connect to Roomzin
	// -------------------------------------------------------------------------
	fmt.Println("[1/8] Connecting to Roomzin...")

	client, err := createClient()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	fmt.Println("Connected successfully")

	// -------------------------------------------------------------------------
	// STEP 2: Create properties and verify existence
	// -------------------------------------------------------------------------
	fmt.Println("[2/8] Creating properties and verifying existence...")

	createdProps := []string{}
	for s := 1; s <= numSegments; s++ {
		segment := fmt.Sprintf("seg_%d", s)
		for p := 1; p <= numPropsPerSegment; p++ {
			propID := fmt.Sprintf("seg_%d_p%d", s, p)

			// Set property with sample data
			lat := 40.7128 + float64(p)*0.001
			lon := -74.0060 + float64(p)*0.001
			amenities := []string{"wifi", "pool"}

			if err := client.SetProp(types.SetPropPayload{
				Segment:      segment,
				Area:         "area_1",
				PropertyID:   propID,
				PropertyType: "hotel",
				Category:     "midrange",
				Stars:        3,
				Latitude:     lat,
				Longitude:    lon,
				Amenities:    amenities,
			}); err != nil {
				log.Fatalf("Failed to create %s: %v", propID, err)
			}

			// Verify property exists
			exists, err := client.PropExist(propID)
			if err != nil {
				log.Fatalf("Failed to check existence for %s: %v", propID, err)
			}
			if !exists {
				log.Fatalf("Property %s does not exist after creation", propID)
			}

			createdProps = append(createdProps, propID)
			fmt.Printf("Created %s\n", propID)
		}
	}
	fmt.Println()

	// -------------------------------------------------------------------------
	// STEP 3: Set room packages and verify rooms/dates
	// -------------------------------------------------------------------------
	fmt.Println("[3/8] Setting room packages and verifying rooms/dates...")

	// Generate dates
	dates := make([]string, numDays)
	for i := range numDays {
		date := time.Now().Add(time.Duration(i+1) * 24 * time.Hour)
		dates[i] = date.Format("2006-01-02")
	}

	// Set packages for all properties
	for s := 1; s <= numSegments; s++ {
		for p := 1; p <= numPropsPerSegment; p++ {
			propID := fmt.Sprintf("seg_%d_p%d", s, p)

			for r := 1; r <= numRoomsPerProp; r++ {
				roomType := fmt.Sprintf("room_%d", r)

				for _, date := range dates {
					avail := uint8(10 + p)
					price := uint32(100 + p*10)
					rateFeatures := []string{"free_cancellation", "free_wifi"}

					if err := client.SetRoomPkg(types.SetRoomPkgPayload{
						PropertyID:   propID,
						RoomType:     roomType,
						Date:         date,
						Availability: &avail,
						FinalPrice:   &price,
						RateFeature:  rateFeatures,
					}); err != nil {
						log.Fatalf("Failed to set package for %s/%s/%s: %v", propID, roomType, date, err)
					}
				}
			}
		}
	}
	fmt.Println("All packages set")

	// Verify room lists for first property
	testProp := "seg_1_p1"
	rooms, err := client.PropRoomList(testProp)
	if err != nil {
		log.Fatalf("Failed to get room list for %s: %v", testProp, err)
	}
	expectedRooms := []string{"room_1", "room_2"}
	if len(rooms) != len(expectedRooms) {
		log.Fatalf("Expected %d rooms, got %d", len(expectedRooms), len(rooms))
	}
	fmt.Printf("%s has rooms: %v\n", testProp, rooms)

	// Verify date lists for first room
	testRoom := "room_1"
	dateList, err := client.PropRoomDateList(types.PropRoomDateListPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
	})
	if err != nil {
		log.Fatalf("Failed to get date list for %s/%s: %v", testProp, testRoom, err)
	}
	if len(dateList) != numDays {
		log.Fatalf("Expected %d dates, got %d", numDays, len(dateList))
	}
	fmt.Printf("%s/%s has dates: %v\n", testProp, testRoom, dateList)

	// Spot check: get a specific room/day
	day, err := client.GetPropRoomDay(types.GetRoomDayRequest{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       dates[0],
	})
	if err != nil {
		log.Fatalf("Failed to get room/day for %s/%s/%s: %v", testProp, testRoom, dates[0], err)
	}
	fmt.Printf("Sample data: %s/%s/%s avail=%d price=%d\n",
		testProp, testRoom, dates[0], day.Availability, day.FinalPrice)
	fmt.Println()

	// -------------------------------------------------------------------------
	// STEP 4: Test SetRoomAvl, IncRoomAvl, DecRoomAvl
	// -------------------------------------------------------------------------
	fmt.Println("[4/8] Testing availability update commands...")

	testDate := dates[0]
	testProp = "seg_1_p1"
	testRoom = "room_1"

	// Get initial availability
	initial, err := client.GetPropRoomDay(types.GetRoomDayRequest{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
	})
	if err != nil {
		log.Fatalf("Failed to get initial availability: %v", err)
	}
	fmt.Printf("Initial: avail=%d, price=%d\n", initial.Availability, initial.FinalPrice)

	// SetRoomAvl
	newAvail := uint8(20)
	_, err = client.SetRoomAvl(types.UpdRoomAvlPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
		Amount:     newAvail,
	})
	if err != nil {
		log.Fatalf("SetRoomAvl failed: %v", err)
	}
	fmt.Printf("SetRoomAvl: %d → %d\n", initial.Availability, newAvail)

	// IncRoomAvl
	incResult, err := client.IncRoomAvl(types.UpdRoomAvlPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
		Amount:     1,
	})
	if err != nil {
		log.Fatalf("IncRoomAvl failed: %v", err)
	}
	fmt.Printf("IncRoomAvl: %d → %d\n", newAvail, incResult)

	// DecRoomAvl
	decResult, err := client.DecRoomAvl(types.UpdRoomAvlPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
		Amount:     1,
	})
	if err != nil {
		log.Fatalf("DecRoomAvl failed: %v", err)
	}
	fmt.Printf("DecRoomAvl: %d → %d\n", incResult, decResult)
	fmt.Println()

	// -------------------------------------------------------------------------
	// STEP 5: Search availability and verify results
	// -------------------------------------------------------------------------
	fmt.Println("[5/8] Searching availability...")

	// Search by segment
	limit := uint64(100)
	results, err := client.SearchAvail(types.SearchAvailPayload{
		Segment:  "seg_1",
		RoomType: testRoom,
		Date:     []string{testDate},
		Limit:    &limit,
	})
	if err != nil {
		log.Fatalf("SearchAvail by segment failed: %v", err)
	}
	fmt.Printf("Found %d properties in seg_1\n", len(results))

	// Search by room type
	results, err = client.SearchAvail(types.SearchAvailPayload{
		Segment:  "seg_1",
		RoomType: "room_1",
		Date:     []string{dates[0]},
		Limit:    &limit,
	})
	if err != nil {
		log.Fatalf("SearchAvail by room type failed: %v", err)
	}
	fmt.Printf("Found %d properties with room_1 in seg_1\n", len(results))

	// Search with filters
	maxPrice := uint32(150)
	results, err = client.SearchAvail(types.SearchAvailPayload{
		Segment:    "seg_1",
		RoomType:   "room_1",
		Date:       []string{dates[0]},
		FinalPrice: &maxPrice,
		Limit:      &limit,
	})
	if err != nil {
		log.Fatalf("SearchAvail with filters failed: %v", err)
	}
	fmt.Printf("Found %d properties with max price %d\n", len(results), maxPrice)
	fmt.Println()

	// -------------------------------------------------------------------------
	// STEP 6: Test deletion commands (in sequence)
	// -------------------------------------------------------------------------
	fmt.Println("[6/8] Testing deletion commands...")

	// 6.1: DelRoomDay
	fmt.Println("Testing DelRoomDay...")
	if err := client.DelRoomDay(types.DelRoomDayRequest{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
	}); err != nil {
		log.Fatalf("DelRoomDay failed: %v", err)
	}

	// Verify date was removed
	dateList, err = client.PropRoomDateList(types.PropRoomDateListPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
	})
	if err != nil {
		log.Fatalf("Failed to get updated date list: %v", err)
	}
	for _, d := range dateList {
		if d == testDate {
			log.Fatalf("Date %s still exists after DelRoomDay", testDate)
		}
	}
	fmt.Println("Date removed successfully")

	// 6.2: DelPropRoom
	fmt.Println("Testing DelPropRoom...")
	if err := client.DelPropRoom(types.DelPropRoomPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
	}); err != nil {
		log.Fatalf("DelPropRoom failed: %v", err)
	}

	// Verify room was removed
	exists, err := client.PropRoomExist(types.PropRoomExistPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
	})
	if err != nil {
		log.Fatalf("Failed to check room existence: %v", err)
	}
	if exists {
		log.Fatalf("Room %s still exists after DelPropRoom", testRoom)
	}
	fmt.Println("Room removed successfully")

	// 6.3: DelProp
	fmt.Println("Testing DelProp...")
	if err := client.DelProp(testProp); err != nil {
		log.Fatalf("DelProp failed: %v", err)
	}

	// Verify property was removed
	exists, err = client.PropExist(testProp)
	if err != nil {
		log.Fatalf("Failed to check property existence: %v", err)
	}
	if exists {
		log.Fatalf("Property %s still exists after DelProp", testProp)
	}
	fmt.Println("Property removed successfully")

	// 6.4: DelSegment
	fmt.Println("Testing DelSegment...")
	if err := client.DelSegment("seg_1"); err != nil {
		log.Fatalf("DelSegment failed: %v", err)
	}

	// Verify segment was removed
	props, err := client.SearchProp(types.SearchPropPayload{
		Segment: "seg_1",
	})
	if err != nil {
		log.Fatalf("Failed to search segment: %v", err)
	}
	if len(props) > 0 {
		log.Fatalf("Segment seg_1 still has %d properties", len(props))
	}
	fmt.Println("Segment removed successfully")
	fmt.Println()

	// -------------------------------------------------------------------------
	// STEP 7: Additional verification (optional)
	// -------------------------------------------------------------------------
	fmt.Println("[7/8] Running additional verification...")

	// Check GetSegments
	segments, err := client.GetSegments()
	if err != nil {
		log.Fatalf("GetSegments failed: %v", err)
	}
	fmt.Printf("Remaining segments: %v\n", segments)

	// Check GetCodecs
	codecs, err := client.GetCodecs()
	if err != nil {
		log.Fatalf("GetCodecs failed: %v", err)
	}
	fmt.Printf("Available rate features: %v\n", codecs.RateFeatures)
	fmt.Println()

	// -------------------------------------------------------------------------
	// STEP 8: Clean up remaining data
	// -------------------------------------------------------------------------
	fmt.Println("[8/8] Cleaning up...")

	// Delete seg_2 (which still has data)
	if err := client.DelSegment("seg_2"); err != nil {
		log.Printf("Warning: Failed to delete seg_2: %v", err)
	} else {
		fmt.Println("Cleaned up seg_2")
	}

	fmt.Println("\nExample completed successfully!")
}
