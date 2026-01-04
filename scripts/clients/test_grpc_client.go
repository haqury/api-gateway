package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "api-gateway/internal/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Подключаемся к gRPC серверу
	conn, err := grpc.Dial(
		"localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewVideoStreamServiceClient(conn)

	// Тест 1: StartStream
	fmt.Println("Test 1: Starting stream...")
	startResp, err := client.StartStream(context.Background(), &pb.StartStreamRequest{
		ClientId:   "test_client_grpc",
		UserId:     "user_001",
		CameraName: "test_camera",
		Filename:   "test_stream.mp4",
	})
	if err != nil {
		log.Fatalf("StartStream failed: %v", err)
	}
	fmt.Printf("Stream started: %s\n", startResp.StreamId)

	// Тест 2: SendFrame (единичный кадр)
	fmt.Println("\nTest 2: Sending single frame...")
	frameResp, err := client.SendFrame(context.Background(), &pb.SendFrameRequest{
		StreamId: startResp.StreamId,
		ClientId: "test_client_grpc",
		UserName: "Test User",
		Frame: &pb.VideoFrame{
			FrameId:   "test_frame_1",
			FrameData: []byte("fake_frame_data"),
			Timestamp: time.Now().Unix(),
			Width:     1920,
			Height:    1080,
			Format:    "jpeg",
		},
	})
	if err != nil {
		log.Fatalf("SendFrame failed: %v", err)
	}
	fmt.Printf("Frame sent: %s\n", frameResp.Message)

	// Тест 3: GetStreamStats
	fmt.Println("\nTest 3: Getting stream stats...")
	stats, err := client.GetStreamStats(context.Background(), &pb.GetStreamStatsRequest{
		StreamId: startResp.StreamId,
		ClientId: "test_client_grpc",
	})
	if err != nil {
		log.Fatalf("GetStreamStats failed: %v", err)
	}
	fmt.Printf("Stats: %d frames, %d bytes, %.2f fps\n",
		stats.FramesReceived, stats.BytesReceived, stats.AverageFps)

	// Тест 4: StopStream
	fmt.Println("\nTest 4: Stopping stream...")
	stopResp, err := client.StopStream(context.Background(), &pb.StopStreamRequest{
		StreamId: startResp.StreamId,
		ClientId: "test_client_grpc",
		Filename: "test_stream.mp4",
		EndTime:  time.Now().Unix(),
		FileSize: 1024 * 1024, // 1MB
	})
	if err != nil {
		log.Fatalf("StopStream failed: %v", err)
	}
	fmt.Printf("Stream stopped: %s\n", stopResp.Message)

	fmt.Println("\n✅ All gRPC tests completed successfully!")
}
