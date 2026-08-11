package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/m-javani/roomzin-go/pkg/client"
	"github.com/m-javani/roomzin-go/pkg/types"
)

// ============================================================================
// CONFIGURATION
// ============================================================================

// Change this to "cluster" to test against a Roomzin cluster via router
// or "standalone" for direct connection to a standalone server
const mode = "cluster"

// Standalone configuration
const (
	standaloneHost = "127.0.0.1"
	standalonePort = 7777
	timeout        = 5 * time.Second
)

// Cluster configuration (router address)
const (
	routerHost = "router.example.com" // or IP address
	routerPort = 7777
)

// Test data parameters
const (
	numSegments        = 2
	numPropsPerSegment = 1000
	numRoomsPerProp    = 2
	numDays            = 3
)

// ============================================================================
// CLIENT CREATION
// ============================================================================

func createClient() (*client.Client, error) {
	if mode == "standalone" {
		cfg, err := client.NewConfigBuilder().
			WithAddr(standaloneHost).
			WithPort(standalonePort).
			WithTimeout(timeout).
			WithKeepAlive(30 * time.Second).
			WithMode(client.StandaloneMode).
			Build()
		if err != nil {
			return nil, err
		}
		return client.New(&cfg)
	}

	// Cluster mode - connect to router
	cfg, err := client.NewConfigBuilder().
		WithAddr(routerHost).
		WithPort(routerPort).
		WithTimeout(30 * time.Second).
		WithKeepAlive(30 * time.Second).
		WithMode(client.ClusterMode).
		Build()
	if err != nil {
		return nil, err
	}
	return client.New(&cfg)
}

// ============================================================================
// MAIN FUNCTION - CLEAR LINEAR FLOW
// ============================================================================

func main() {
	ctx := context.Background()
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

	// -------------------------------------------------------------------------
	// STEP 2: Create properties and verify existence
	// -------------------------------------------------------------------------
	fmt.Println("[2/8] SetProp...")

	createdProps := []string{}
	for s := 1; s <= numSegments; s++ {
		segment := fmt.Sprintf("seg_%d", s)
		for p := 1; p <= numPropsPerSegment; p++ {
			propID := fmt.Sprintf("seg_%d_p%d", s, p)

			// Set property with sample data
			lat := 40.7128 + float64(p)*0.001
			lon := -74.0060 + float64(p)*0.001
			amenities := []string{"wifi", "pool"}

			if err := client.SetProp(ctx, segment, types.SetPropPayload{
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

			createdProps = append(createdProps, propID)
		}
	}

	// check PropExist
	p1 := createdProps[len(createdProps)-1]
	segment := "seg_1"
	if err := waitForCondition(2*time.Second, func() (bool, error) {
		return client.PropExist(ctx, segment, p1)
	}); err != nil {
		log.Fatalf("Property %s did not become available: %v", p1, err)
	}

	// -------------------------------------------------------------------------
	// STEP 3: Set room packages and verify rooms/dates
	// -------------------------------------------------------------------------
	fmt.Println("[3/8] SetRoomPkg...")

	// Generate dates
	dates := make([]string, numDays)
	for i := range numDays {
		date := time.Now().Add(time.Duration(i+1) * 24 * time.Hour)
		dates[i] = date.Format("2006-01-02")
	}

	// Set packages for all properties
	for s := 1; s <= numSegments; s++ {
		segment := fmt.Sprintf("seg_%d", s)
		for p := 1; p <= numPropsPerSegment; p++ {
			propID := fmt.Sprintf("seg_%d_p%d", s, p)

			for r := 1; r <= numRoomsPerProp; r++ {
				roomType := fmt.Sprintf("room_%d", r)

				for _, date := range dates {
					avail := uint8(10 + p)
					price := uint32(100 + p*10)
					rateFeatures := []string{"free_cancellation", "free_wifi"}

					if err := client.SetRoomPkg(ctx, segment, types.SetRoomPkgPayload{
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

	// Verify room lists for first property
	testProp := "seg_1_p1"
	rooms, err := client.PropRoomList(ctx, segment, testProp)
	if err != nil {
		log.Fatalf("Failed to get room list for %s: %v", testProp, err)
	}
	expectedRooms := []string{"room_1", "room_2"}
	if len(rooms) != len(expectedRooms) {
		log.Fatalf("Expected %d rooms, got %d", len(expectedRooms), len(rooms))
	}

	// Verify date lists for first room
	testRoom := "room_1"
	dateList, err := client.PropRoomDateList(ctx, segment, types.PropRoomDateListPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
	})
	if err != nil {
		log.Fatalf("Failed to get date list for %s/%s: %v", testProp, testRoom, err)
	}
	if len(dateList) != numDays {
		log.Fatalf("Expected %d dates, got %d", numDays, len(dateList))
	}
	fmt.Printf("	PropRoomDateList: %+v\n", dateList)

	// Spot check: get a specific room/day
	_, err = client.GetPropRoomDay(ctx, segment, types.GetRoomDayRequest{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       dates[0],
	})
	if err != nil {
		log.Fatalf("Failed to get room/day for %s/%s/%s: %v", testProp, testRoom, dates[0], err)
	}

	// -------------------------------------------------------------------------
	// STEP 4: Test SetRoomAvl, IncRoomAvl, DecRoomAvl
	// -------------------------------------------------------------------------
	fmt.Println("[4/8] Update Availability...")

	testDate := dates[0]
	testProp = "seg_1_p1"
	testRoom = "room_1"

	// Get initial availability
	initial, err := client.GetPropRoomDay(ctx, segment, types.GetRoomDayRequest{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
	})
	if err != nil {
		log.Fatalf("Failed to get initial availability: %v", err)
	}
	fmt.Printf("	GetPropRoomDay: avail=%d, price=%d\n", initial.Availability, initial.FinalPrice)

	// SetRoomAvl
	newAvail := uint8(20)
	_, err = client.SetRoomAvl(ctx, segment, types.UpdRoomAvlPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
		Amount:     newAvail,
	})
	if err != nil {
		log.Fatalf("SetRoomAvl failed: %v", err)
	}
	fmt.Printf("	SetRoomAvl: %d → %d\n", initial.Availability, newAvail)

	// IncRoomAvl
	incResult, err := client.IncRoomAvl(ctx, segment, types.UpdRoomAvlPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
		Amount:     1,
	})
	if err != nil {
		log.Fatalf("IncRoomAvl failed: %v", err)
	}
	fmt.Printf("	IncRoomAvl: %d → %d\n", newAvail, incResult)

	// DecRoomAvl
	decResult, err := client.DecRoomAvl(ctx, segment, types.UpdRoomAvlPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
		Amount:     1,
	})
	if err != nil {
		log.Fatalf("DecRoomAvl failed: %v", err)
	}
	fmt.Printf("	DecRoomAvl: %d → %d\n", incResult, decResult)

	// -------------------------------------------------------------------------
	// STEP 5: Search availability and verify results
	// -------------------------------------------------------------------------
	fmt.Println("[5/8] SearchAvail...")

	limit := uint64(100)
	maxPrice := uint32(150)
	results, err := client.SearchAvail(ctx, "seg_1", types.SearchAvailPayload{
		Segment:    "seg_1", // Still needed for the payload itself
		RoomType:   "room_1",
		Date:       []string{dates[0]},
		FinalPrice: &maxPrice,
		Limit:      &limit,
	})
	if err != nil {
		log.Fatalf("SearchAvail with filters failed: %v", err)
	}
	fmt.Printf("	Found %d properties with max price %d\n", len(results), maxPrice)

	// -------------------------------------------------------------------------
	// STEP 6: Test deletion commands (in sequence)
	// -------------------------------------------------------------------------
	fmt.Println("[6/8] Deletion commands...")

	// 6.1: DelRoomDay
	fmt.Println("	DelRoomDay...")
	if err := client.DelRoomDay(ctx, segment, types.DelRoomDayRequest{
		PropertyID: testProp,
		RoomType:   testRoom,
		Date:       testDate,
	}); err != nil {
		log.Fatalf("DelRoomDay failed: %v", err)
	}

	// Verify date was removed
	if err := waitForCondition(2*time.Second, func() (bool, error) {
		dateList, err := client.PropRoomDateList(ctx, segment, types.PropRoomDateListPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
		})
		if err != nil {
			return false, err
		}
		if slices.Contains(dateList, testDate) {
			return false, nil
		}
		return true, nil
	}); err != nil {
		log.Fatalf("Date %s still exists after DelRoomDay: %v", testDate, err)
	}

	// 6.2: DelPropRoom
	fmt.Println("	DelPropRoom...")
	if err := client.DelPropRoom(ctx, segment, types.DelPropRoomPayload{
		PropertyID: testProp,
		RoomType:   testRoom,
	}); err != nil {
		log.Fatalf("DelPropRoom failed: %v", err)
	}

	// Verify room was removed
	if err := waitForCondition(2*time.Second, func() (bool, error) {
		exists, err := client.PropRoomExist(ctx, segment, types.PropRoomExistPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
		})
		return !exists, err
	}); err != nil {
		log.Fatalf("Room %s still exists after DelPropRoom: %v", testRoom, err)
	}

	// 6.3: DelProp
	fmt.Println("	DelProp...")
	if err := client.DelProp(ctx, segment, testProp); err != nil {
		log.Fatalf("DelProp failed: %v", err)
	}

	// Verify property was removed
	if err := waitForCondition(2*time.Second, func() (bool, error) {
		exists, err := client.PropExist(ctx, segment, testProp)
		return !exists, err
	}); err != nil {
		log.Fatalf("Property %s still exists after DelProp: %v", testProp, err)
	}

	// 6.4: DelSegment
	fmt.Println("	DelSegment...")
	if err := client.DelSegment(ctx, "seg_1"); err != nil {
		log.Fatalf("DelSegment failed: %v", err)
	}

	// Verify segment was removed
	if err := waitForCondition(2*time.Second, func() (bool, error) {
		props, err := client.SearchProp(ctx, "seg_1", types.SearchPropPayload{
			Segment: "seg_1",
		})
		if err != nil {
			return false, err
		}
		return len(props) == 0, nil
	}); err != nil {
		log.Fatalf("Segment seg_1 still has properties: %v", err)
	}

	// -------------------------------------------------------------------------
	// STEP 7: Clean up remaining data
	// -------------------------------------------------------------------------
	fmt.Println("[7/7] Cleaning up...")

	// Delete seg_2 (which still has data)
	if err := client.DelSegment(ctx, "seg_2"); err != nil {
		log.Printf("Warning: Failed to delete seg_2: %v", err)
	} else {
		fmt.Println("	Cleaned up seg_2")
	}

	fmt.Println("All completed successfully!")
}

func waitForCondition(timeout time.Duration, condition func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		ok, err := condition()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("condition not met within %v", timeout)
		}
		<-ticker.C
	}
}
