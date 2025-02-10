package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"

	pb "chat_app/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	port  = flag.Int("port", 50051, "The server port")
	users = []*pb.User{
		{
			Id:       "1",
			Username: "John",
		},
		{
			Id:       "2",
			Username: "Dan",
		},
		{
			Id:       "3",
			Username: "Hannah",
		},
	}
	channels = []*pb.Channel{
		{Id: "1", Name: "Games"},
		{Id: "2", Name: "Music"},
		{Id: "3", Name: "Movies"},
	}
	activeStreams = make(map[string][]pb.ChatService_StartMessagingServer)
)

type chat_service_server struct {
	pb.UnimplementedChatServiceServer
}

func findUserById(userId string) *pb.User {
	for _, user := range users {
		if user.Id == userId {
			return user
		}
	}
	return nil
}

func findChannelById(channelId string) *pb.Channel {
	for _, channel := range channels {
		if channel.Id == channelId {
			return channel
		}
	}
	return nil
}

func findMemberById(channelId string, userId string) *pb.User {
	channel := findChannelById(channelId)
	if channel == nil {
		return nil
	}

	for _, member := range channel.Members {
		if member.Id == userId {
			return member
		}
	}
	return nil
}

func isUserInChannel(userId string, members []*pb.User) bool {
	for _, member := range members {
		if member.Id == userId {
			return true
		}
	}
	return false
}

func (s *chat_service_server) JoinChannel(ctx context.Context, request *pb.UserChannelRequest) (*pb.JoinChannelAck, error) {
	log.Printf("Joining channel: %v", request)

	channel := findChannelById(request.ChannelId)
	if channel == nil {
		return &pb.JoinChannelAck{
			Joined:       false,
			ErrorMessage: "Channel not found",
		}, status.Error(codes.NotFound, "Channel not found")
	}

	member := findMemberById(channel.Id, request.UserId)
	if member != nil {
		log.Printf("User is already a member of this channel: %v", request.UserId)
		return &pb.JoinChannelAck{
			Joined:       false,
			ErrorMessage: "User is already a member of this channel",
		}, status.Error(codes.InvalidArgument, "User is already a member of this channel")
	}

	user := findUserById(request.UserId)
	if user == nil {
		return &pb.JoinChannelAck{
			Joined:       false,
			ErrorMessage: "User not found",
		}, status.Error(codes.NotFound, "User not found")
	}

	channel.Members = append(channel.Members, user)
	log.Printf("User successfully added to channel: %v", request.UserId)
	return &pb.JoinChannelAck{
		Joined: true,
	}, nil
}

func broadcastMessageToChannel(msg *pb.ChatMessage) {
	for _, clientStream := range activeStreams[msg.ChannelId] {
		err := clientStream.Send(msg)
		if err != nil {
			log.Printf("Error sending message to client: %v", err)
			continue
		}
	}
}

func (s *chat_service_server) StartMessaging(stream pb.ChatService_StartMessagingServer) error {
	log.Printf("Joining channel...")
	ctx := stream.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "Missing metadata")
	}

	if len(md["user_id"]) == 0 || len(md["channel_id"]) == 0 {
		return status.Error(codes.InvalidArgument, "user_id or channel_id not provided")
	}

	userId := md["user_id"][0]
	channelId := md["channel_id"][0]

	log.Printf("User ID: %s", userId)
	log.Printf("Channel ID: %s", channelId)

	user := findUserById(userId)
	if user == nil {
		return status.Error(codes.NotFound, "User not found")
	}

	channel := findChannelById(channelId)
	if channel == nil {
		return status.Error(codes.NotFound, "Channel not found")
	}

	foundUserInChannel := isUserInChannel(userId, channel.Members)
	if !foundUserInChannel {
		return status.Error(codes.PermissionDenied, "User not authorized in this channel")
	}

	log.Printf("User successfully authorised!")
	activeStreams[channelId] = append(activeStreams[channelId], stream)

	defer func() {
		for i, s := range activeStreams[channelId] {
			if s == stream {
				activeStreams[channelId] = append(activeStreams[channelId][:i], activeStreams[channelId][i+1:]...)
				break
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var sender *pb.User
		for _, user := range users {
			if user.Id == msg.SenderId {
				sender = user
				break
			}
		}

		log.Printf("Message from %s: %s", sender.Username, msg.Content)

		broadcastMessageToChannel(msg)
	}
}

func (s *chat_service_server) SendMessage(ctx context.Context, msg *pb.ChatMessage) (*pb.MessageAck, error) {
	sender := findUserById(msg.SenderId)
	if sender == nil {
		return &pb.MessageAck{Sent: false, ErrorMessage: "User not found"}, status.Error(codes.NotFound, "User not found")
	}

	broadcastMessageToChannel(msg)

	return &pb.MessageAck{Sent: true}, nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterChatServiceServer(s, &chat_service_server{})
	log.Printf("Server listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
