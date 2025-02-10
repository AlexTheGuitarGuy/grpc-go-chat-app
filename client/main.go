package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"

	pb "chat_app/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var addr = flag.String("addr", "localhost:50051", "The address to connect to")

func receiveMessages(stream pb.ChatService_StartMessagingClient, userId string) {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return
		}

		if err != nil {
			log.Printf("Error receiving message: %v", err)
			return
		}

		if userId != msg.SenderId {
			log.Printf("Received message from %s: %s", msg.SenderId, msg.Content)
		}

	}
}

func main() {
	flag.Parse()

	var userId string
	fmt.Print("Enter user: ")
	fmt.Scanln(&userId)

	var channelId string
	fmt.Print("Enter channel: ")
	fmt.Scanln(&channelId)

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Couldn't connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("user_id", userId, "channel_id", channelId),
	)

	joinAck, err := client.JoinChannel(ctx, &pb.UserChannelRequest{
		UserId:    userId,
		ChannelId: channelId,
	})
	if err != nil {
		log.Fatalf("Error joining channel: %v", err)
	}
	if joinAck.Joined == false {
		log.Fatalf("Error joining channel: %s", joinAck.ErrorMessage)
	}

	stream, err := client.StartMessaging(ctx)
	if err != nil {
		log.Fatalf("Error starting messaging: %v", err)
	}

	go receiveMessages(stream, userId)

	for {
		var message string
		fmt.Print("Enter message: ")
		fmt.Scanln(&message)

		if message == "exit" {
			break
		}

		err := stream.Send(&pb.ChatMessage{
			SenderId:  userId,
			Content:   message,
			ChannelId: channelId,
		})
		if err != nil {
			log.Printf("Error sending message: %v", err)
			continue
		}

		log.Printf("Sent message: %s", message)
	}

	if err := stream.CloseSend(); err != nil {
		log.Fatalf("Error closing stream: %v", err)
	}
}
