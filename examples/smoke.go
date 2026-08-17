package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
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
	routerHost = "127.0.0.1" // or IP address
	routerPort = 9200        // edge router port
)

// Test data parameters - matches generator.py
const (
	numSegments        = 4  // segments per shard (generator: SEGMENTS_PER_SHARD = 4)
	numPropsPerSegment = 10 // properties per segment (PROPERTIES_PER_SEGMENT = 10)
	numRoomsPerProp    = 4  // room types per property (ROOM_TYPES_PER_PROPERTY = 4)
	numDays            = 10 // days (DEFAULT_DAYS = 10)
	shardIdx           = 1  // which shard to test (1 or 2)
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
// TIMING HELPERS
// ============================================================================

type StepTiming struct {
	name         string
	duration     time.Duration
	requestCount int
}

var timings []StepTiming

func timeStep(name string, requestCount int, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	timings = append(timings, StepTiming{
		name:         name,
		duration:     duration,
		requestCount: requestCount,
	})

	if err != nil {
		fmt.Printf("  ❌ %s failed after %v\n", name, duration)
	} else {
		fmt.Printf("  ✅ %s completed in %v (%d requests)\n", name, duration, requestCount)
	}
	return err
}

func printSummary() {
	fmt.Println("\n" + "=" + strings.Repeat("=", 60))
	fmt.Println("  TIMING SUMMARY")
	fmt.Println("=" + strings.Repeat("=", 60))

	var totalTime time.Duration
	var totalRequests int

	for _, t := range timings {
		totalTime += t.duration
		totalRequests += t.requestCount
		fmt.Printf("  %-25s %10v  %4d requests\n", t.name+":", t.duration, t.requestCount)
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  %-25s %10v  %4d requests\n", "TOTAL:", totalTime, totalRequests)
	fmt.Printf("  %-25s %10v\n", "Avg per request:", totalTime/time.Duration(totalRequests))
	fmt.Println("=" + strings.Repeat("=", 60))
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

	codecs, err := client.GetCodecs()
	if err != nil {
		log.Fatalf("Failed to get codecs: %v", err)
	}
	fmt.Printf("codecs %+v \n", codecs.RateFeatures)

	// -------------------------------------------------------------------------
	// STEP 2: Create properties and verify existence
	// -------------------------------------------------------------------------
	fmt.Println("\n[2/8] SetProp...")

	err = timeStep("SetProp", numSegments*numPropsPerSegment, func() error {
		createdProps := []string{}

		for s := 1; s <= numSegments; s++ {
			segment := fmt.Sprintf("segment_%d", s)

			for p := 1; p <= numPropsPerSegment; p++ {
				propID := fmt.Sprintf("s%d_seg%d_p%d", shardIdx, s, p)

				lat := 40.7128 + float64(p)*0.001
				lon := -74.0060 + float64(p)*0.001
				amenities := []string{"wifi", "pool"}

				if err := client.SetProp(ctx, segment, types.SetPropPayload{
					Segment:      segment,
					Area:         fmt.Sprintf("area_%d_%d", shardIdx, s),
					PropertyID:   propID,
					PropertyType: "hotel",
					Category:     "midrange",
					Stars:        3,
					Latitude:     lat,
					Longitude:    lon,
					Amenities:    amenities,
				}); err != nil {
					return fmt.Errorf("failed to create %s: %v", propID, err)
				}

				createdProps = append(createdProps, propID)
			}
		}

		// check PropExist
		p1 := createdProps[0]
		segment := "segment_1"
		if err := waitForCondition(2*time.Second, func() (bool, error) {
			return client.PropExist(ctx, segment, p1)
		}); err != nil {
			return fmt.Errorf("property %s did not become available: %v", p1, err)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("SetProp failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// STEP 3: Set room packages and verify rooms/dates
	// -------------------------------------------------------------------------
	fmt.Println("\n[3/8] SetRoomPkg...")

	err = timeStep("SetRoomPkg", numSegments*numPropsPerSegment*numRoomsPerProp*numDays, func() error {
		// Generate dates - start from today (matches generator)
		dates := make([]string, numDays)
		for i := range numDays {
			date := time.Now().Add(time.Duration(i) * 24 * time.Hour)
			dates[i] = date.Format("2006-01-02")
		}

		// Set packages for all properties
		for s := 1; s <= numSegments; s++ {
			segment := fmt.Sprintf("segment_%d", s)

			for p := 1; p <= numPropsPerSegment; p++ {
				propID := fmt.Sprintf("s%d_seg%d_p%d", shardIdx, s, p)

				for r := 1; r <= numRoomsPerProp; r++ {
					roomType := fmt.Sprintf("room%d", r)

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
							return fmt.Errorf("failed to set package for %s/%s/%s: %v", propID, roomType, date, err)
						}
					}
				}
			}
		}

		// Verify room lists for first property
		testProp := fmt.Sprintf("s%d_seg1_p1", shardIdx)
		segment := "segment_1"

		rooms, err := client.PropRoomList(ctx, segment, testProp)
		if err != nil {
			return fmt.Errorf("failed to get room list for %s: %v", testProp, err)
		}

		expectedRooms := []string{"room1", "room2", "room3", "room4"}
		if len(rooms) != len(expectedRooms) {
			return fmt.Errorf("expected %d rooms, got %d", len(expectedRooms), len(rooms))
		}

		// Verify date lists for first room
		testRoom := "room1"
		dateList, err := client.PropRoomDateList(ctx, segment, types.PropRoomDateListPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
		})
		if err != nil {
			return fmt.Errorf("failed to get date list for %s/%s: %v", testProp, testRoom, err)
		}

		if len(dateList) != numDays {
			return fmt.Errorf("expected %d dates, got %d", numDays, len(dateList))
		}
		fmt.Printf("        PropRoomDateList: %+v\n", dateList)

		// Spot check: get a specific room/day
		_, err = client.GetPropRoomDay(ctx, segment, types.GetRoomDayRequest{
			PropertyID: testProp,
			RoomType:   testRoom,
			Date:       dates[0],
		})
		if err != nil {
			return fmt.Errorf("failed to get room/day for %s/%s/%s: %v", testProp, testRoom, dates[0], err)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("SetRoomPkg failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// STEP 4: Test SetRoomAvl, IncRoomAvl, DecRoomAvl
	// -------------------------------------------------------------------------
	fmt.Println("\n[4/8] Update Availability...")

	err = timeStep("Update Availability", 4, func() error {
		testDate := time.Now().Format("2006-01-02")
		testProp := fmt.Sprintf("s%d_seg1_p1", shardIdx)
		testRoom := "room1"
		segment := "segment_1"

		// Get initial availability
		initial, err := client.GetPropRoomDay(ctx, segment, types.GetRoomDayRequest{
			PropertyID: testProp,
			RoomType:   testRoom,
			Date:       testDate,
		})
		if err != nil {
			return fmt.Errorf("failed to get initial availability: %v", err)
		}
		fmt.Printf("        GetPropRoomDay: avail=%d, price=%d\n", initial.Availability, initial.FinalPrice)

		// SetRoomAvl
		newAvail := uint8(20)
		_, err = client.SetRoomAvl(ctx, segment, types.UpdRoomAvlPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
			Date:       testDate,
			Amount:     newAvail,
		})
		if err != nil {
			return fmt.Errorf("SetRoomAvl failed: %v", err)
		}
		fmt.Printf("        SetRoomAvl: %d → %d\n", initial.Availability, newAvail)

		// IncRoomAvl
		incResult, err := client.IncRoomAvl(ctx, segment, types.UpdRoomAvlPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
			Date:       testDate,
			Amount:     1,
		})
		if err != nil {
			return fmt.Errorf("IncRoomAvl failed: %v", err)
		}
		fmt.Printf("        IncRoomAvl: %d → %d\n", newAvail, incResult)

		// DecRoomAvl
		decResult, err := client.DecRoomAvl(ctx, segment, types.UpdRoomAvlPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
			Date:       testDate,
			Amount:     1,
		})
		if err != nil {
			return fmt.Errorf("DecRoomAvl failed: %v", err)
		}
		fmt.Printf("        DecRoomAvl: %d → %d\n", incResult, decResult)

		return nil
	})
	if err != nil {
		log.Fatalf("Update Availability failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// STEP 5: Search availability and verify results
	// -------------------------------------------------------------------------
	fmt.Println("\n[5/8] SearchAvail...")

	err = timeStep("SearchAvail", 1, func() error {
		dates := make([]string, 1)
		dates[0] = time.Now().Format("2006-01-02")

		limit := uint64(100)
		maxPrice := uint32(150)
		results, err := client.SearchAvail(ctx, "segment_1", types.SearchAvailPayload{
			Segment:    "segment_1",
			RoomType:   "room1",
			Date:       []string{dates[0]},
			FinalPrice: &maxPrice,
			Limit:      &limit,
		})
		if err != nil {
			return fmt.Errorf("SearchAvail with filters failed: %v", err)
		}
		fmt.Printf("        Found %d properties with max price %d\n", len(results), maxPrice)
		return nil
	})
	if err != nil {
		log.Fatalf("SearchAvail failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// STEP 6: Test deletion commands (in sequence)
	// -------------------------------------------------------------------------
	fmt.Println("\n[6/8] Deletion commands...")

	err = timeStep("Deletion", 8, func() error {
		segment := "segment_1"
		testProp := fmt.Sprintf("s%d_seg1_p1", shardIdx)
		testRoom := "room1"
		testDate := time.Now().Format("2006-01-02")

		// 6.1: DelRoomDay
		fmt.Println("        DelRoomDay...")
		if err := client.DelRoomDay(ctx, segment, types.DelRoomDayRequest{
			PropertyID: testProp,
			RoomType:   testRoom,
			Date:       testDate,
		}); err != nil {
			return fmt.Errorf("DelRoomDay failed: %v", err)
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
			return fmt.Errorf("date %s still exists after DelRoomDay: %v", testDate, err)
		}

		// 6.2: DelPropRoom
		fmt.Println("        DelPropRoom...")
		if err := client.DelPropRoom(ctx, segment, types.DelPropRoomPayload{
			PropertyID: testProp,
			RoomType:   testRoom,
		}); err != nil {
			return fmt.Errorf("DelPropRoom failed: %v", err)
		}

		// Verify room was removed
		if err := waitForCondition(2*time.Second, func() (bool, error) {
			exists, err := client.PropRoomExist(ctx, segment, types.PropRoomExistPayload{
				PropertyID: testProp,
				RoomType:   testRoom,
			})
			return !exists, err
		}); err != nil {
			return fmt.Errorf("room %s still exists after DelPropRoom: %v", testRoom, err)
		}

		// 6.3: DelProp
		fmt.Println("        DelProp...")
		if err := client.DelProp(ctx, segment, testProp); err != nil {
			return fmt.Errorf("DelProp failed: %v", err)
		}

		// Verify property was removed
		if err := waitForCondition(2*time.Second, func() (bool, error) {
			exists, err := client.PropExist(ctx, segment, testProp)
			return !exists, err
		}); err != nil {
			return fmt.Errorf("property %s still exists after DelProp: %v", testProp, err)
		}

		// 6.4: DelSegment
		fmt.Println("        DelSegment...")
		if err := client.DelSegment(ctx, "segment_1"); err != nil {
			return fmt.Errorf("DelSegment failed: %v", err)
		}

		// Verify segment was removed
		if err := waitForCondition(2*time.Second, func() (bool, error) {
			props, err := client.SearchProp(ctx, "segment_1", types.SearchPropPayload{
				Segment: "segment_1",
			})
			if err != nil {
				return false, err
			}
			return len(props) == 0, nil
		}); err != nil {
			return fmt.Errorf("segment segment_1 still has properties: %v", err)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Deletion failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// STEP 7: Clean up remaining data
	// -------------------------------------------------------------------------
	fmt.Println("\n[7/8] Cleaning up...")

	err = timeStep("Cleanup", 3, func() error {
		for s := 2; s <= numSegments; s++ {
			seg := fmt.Sprintf("segment_%d", s)
			if err := client.DelSegment(ctx, seg); err != nil {
				log.Printf("Warning: Failed to delete %s: %v", seg, err)
			} else {
				fmt.Printf("        Cleaned up %s\n", seg)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Cleanup had issues: %v", err)
	}

	// -------------------------------------------------------------------------
	// SUMMARY
	// -------------------------------------------------------------------------
	printSummary()
	fmt.Println("\n✅ All completed successfully!")
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
