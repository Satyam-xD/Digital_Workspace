package socket

import (
	"log"
	"net/http"
	"sync"

	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/socket.io/v2/socket"
)

type Participant struct {
	UserID   string `json:"id"`
	UserName string `json:"name"`
	SocketID string `json:"-"`
}

type RingingCall struct {
	RecipientID string `json:"recipientId"`
	CallerID    string `json:"callerId"`
	CallerName  string `json:"callerName"`
	IsVideo     bool   `json:"isVideo"`
}

type SocketManager struct {
	mu           sync.RWMutex
	Server       *socket.Server
	OnlineUsers  map[string]socket.SocketId         // userId -> socketId
	VideoRooms   map[string]map[string]*Participant // roomId -> map[userId]*Participant
	RingingCalls map[socket.SocketId]*RingingCall   // callerSocketId -> RingingCall
}

var Manager *SocketManager

// InitSocketServer initializes the Socket.IO v4 server and handlers
func InitSocketServer() (*socket.Server, http.Handler) {
	opts := socket.DefaultServerOptions()
	opts.SetAllowEIO3(true)
	opts.SetCors(&types.Cors{
		Origin:      true,
		Credentials: true,
	})
	opts.SetTransports(types.NewSet("polling", "websocket"))

	io := socket.NewServer(nil, opts)

	Manager = &SocketManager{
		Server:       io,
		OnlineUsers:  make(map[string]socket.SocketId),
		VideoRooms:   make(map[string]map[string]*Participant),
		RingingCalls: make(map[socket.SocketId]*RingingCall),
	}

	io.On("connection", func(clients ...any) {
		client := clients[0].(*socket.Socket)
		socketID := client.Id()

		// Send "me" event
		client.Emit("me", string(socketID))

		// setup event: user logging in or refreshing
		client.On("setup", func(args ...any) {
			if len(args) == 0 {
				return
			}
			userData, ok := args[0].(map[string]any)
			if !ok {
				return
			}

			var userID string
			if id, ok := userData["_id"].(string); ok && id != "" {
				userID = id
			} else if id, ok := userData["id"].(string); ok && id != "" {
				userID = id
			}

			if userID != "" {
				client.Join(socket.Room(userID))

				Manager.mu.Lock()
				Manager.OnlineUsers[userID] = socketID
				if role, ok := userData["role"].(string); ok && role == "master_admin" {
					client.Join("platform_admin")
				}
				onlineKeys := make([]string, 0, len(Manager.OnlineUsers))
				for k := range Manager.OnlineUsers {
					onlineKeys = append(onlineKeys, k)
				}
				Manager.mu.Unlock()

				io.Emit("onlineUsers", onlineKeys)
				client.Emit("connected")
			}
		})

		// setup_dashboard: join team rooms
		client.On("setup_dashboard", func(args ...any) {
			if len(args) == 0 {
				return
			}
			payload, ok := args[0].(map[string]any)
			if !ok {
				return
			}
			if teamIDs, ok := payload["teamIds"].([]any); ok {
				for _, tid := range teamIDs {
					if idStr, ok := tid.(string); ok {
						client.Join(socket.Room("team_" + idStr))
					}
				}
			}
		})

		// joinRoom: video call room
		client.On("joinRoom", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, ok := args[0].(map[string]any)
			if !ok {
				return
			}

			roomID, _ := data["roomId"].(string)
			userID, _ := data["userId"].(string)
			userName, _ := data["userName"].(string)
			name, _ := data["name"].(string)

			if userName == "" && name != "" {
				userName = name
			}

			if roomID != "" && userID != "" && userName != "" {
				client.Join(socket.Room(roomID))

				Manager.mu.Lock()
				room, exists := Manager.VideoRooms[roomID]
				if !exists {
					room = make(map[string]*Participant)
					Manager.VideoRooms[roomID] = room
				}
				room[userID] = &Participant{
					UserID:   userID,
					UserName: userName,
					SocketID: string(socketID),
				}

				participants := make([]map[string]string, 0, len(room))
				for _, p := range room {
					participants = append(participants, map[string]string{
						"id":   p.UserID,
						"name": p.UserName,
					})
				}
				Manager.mu.Unlock()

				client.Emit("roomJoined", map[string]any{"participants": participants})
				client.To(socket.Room(roomID)).Emit("userJoinedRoom", map[string]any{
					"userId":       userID,
					"userName":     userName,
					"participants": participants,
				})
			}
		})

		// leaveRoom: video call room
		client.On("leaveRoom", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, ok := args[0].(map[string]any)
			if !ok {
				return
			}
			roomID, _ := data["roomId"].(string)
			userID, _ := data["userId"].(string)

			if roomID != "" {
				Manager.mu.Lock()
				if room, ok := Manager.VideoRooms[roomID]; ok {
					delete(room, userID)
					client.Leave(socket.Room(roomID))

					participants := make([]map[string]string, 0, len(room))
					for _, p := range room {
						participants = append(participants, map[string]string{
							"id":   p.UserID,
							"name": p.UserName,
						})
					}
					if len(room) == 0 {
						delete(Manager.VideoRooms, roomID)
					}
					Manager.mu.Unlock()

					io.To(socket.Room(roomID)).Emit("userLeftRoom", map[string]any{
						"userId":       userID,
						"participants": participants,
					})
				} else {
					Manager.mu.Unlock()
				}
			}
		})

		// WebRTC offer / answer / ice
		client.On("sendOffer", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			to, _ := data["to"].(string)
			offer := data["offer"]
			roomID, _ := data["roomId"].(string)

			Manager.mu.RLock()
			room := Manager.VideoRooms[roomID]
			var recipientSocketID socket.SocketId
			var senderID, senderName string

			if room != nil {
				if r, ok := room[to]; ok {
					recipientSocketID = socket.SocketId(r.SocketID)
				}
				for uid, p := range room {
					if p.SocketID == string(socketID) {
						senderID = uid
						senderName = p.UserName
						break
					}
				}
			}
			Manager.mu.RUnlock()

			if recipientSocketID != "" {
				io.To(socket.Room(recipientSocketID)).Emit("receiveOffer", map[string]any{
					"from":     senderID,
					"fromName": senderName,
					"offer":    offer,
				})
			}
		})

		client.On("sendAnswer", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			to, _ := data["to"].(string)
			answer := data["answer"]
			roomID, _ := data["roomId"].(string)

			Manager.mu.RLock()
			room := Manager.VideoRooms[roomID]
			var recipientSocketID socket.SocketId
			var senderID string

			if room != nil {
				if r, ok := room[to]; ok {
					recipientSocketID = socket.SocketId(r.SocketID)
				}
				for uid, p := range room {
					if p.SocketID == string(socketID) {
						senderID = uid
						break
					}
				}
			}
			Manager.mu.RUnlock()

			if recipientSocketID != "" {
				io.To(socket.Room(recipientSocketID)).Emit("receiveAnswer", map[string]any{
					"from":   senderID,
					"answer": answer,
				})
			}
		})

		client.On("sendIceCandidate", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			to, _ := data["to"].(string)
			candidate := data["candidate"]
			roomID, _ := data["roomId"].(string)

			Manager.mu.RLock()
			room := Manager.VideoRooms[roomID]
			var recipientSocketID socket.SocketId
			var senderID string

			if room != nil {
				if r, ok := room[to]; ok {
					recipientSocketID = socket.SocketId(r.SocketID)
				}
				for uid, p := range room {
					if p.SocketID == string(socketID) {
						senderID = uid
						break
					}
				}
			}
			Manager.mu.RUnlock()

			if recipientSocketID != "" {
				io.To(socket.Room(recipientSocketID)).Emit("receiveIceCandidate", map[string]any{
					"from":      senderID,
					"candidate": candidate,
				})
			}
		})

		client.On("roomMessage", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			roomID, _ := data["roomId"].(string)
			msg := data["message"]

			if roomID != "" {
				client.To(socket.Room(roomID)).Emit("roomMessage", map[string]any{"message": msg})
			}
		})

		// 1-on-1 Calling
		client.On("callUser", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			userToCall, _ := data["userToCall"].(string)
			signalData := data["signalData"]
			from, _ := data["from"].(string)
			name, _ := data["name"].(string)
			isVideo, _ := data["isVideo"].(bool)

			Manager.mu.Lock()
			Manager.RingingCalls[socketID] = &RingingCall{
				RecipientID: userToCall,
				CallerID:    from,
				CallerName:  name,
				IsVideo:     isVideo,
			}
			Manager.mu.Unlock()

			io.To(socket.Room(userToCall)).Emit("callUser", map[string]any{
				"signal":  signalData,
				"from":    from,
				"name":    name,
				"isVideo": isVideo,
			})
		})

		client.On("answerCall", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			to, _ := data["to"].(string)
			signal := data["signal"]

			Manager.mu.Lock()
			for callerID, info := range Manager.RingingCalls {
				if info.RecipientID == to || string(callerID) == to {
					delete(Manager.RingingCalls, callerID)
				}
			}
			Manager.mu.Unlock()

			io.To(socket.Room(to)).Emit("callAccepted", signal)
		})

		client.On("ice-candidate", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			to, _ := data["to"].(string)
			cand := data["candidate"]
			io.To(socket.Room(to)).Emit("ice-candidate", cand)
		})

		client.On("endCall", func(args ...any) {
			if len(args) == 0 {
				return
			}
			data, _ := args[0].(map[string]any)
			to, _ := data["to"].(string)

			Manager.mu.Lock()
			delete(Manager.RingingCalls, socketID)
			Manager.mu.Unlock()

			io.To(socket.Room(to)).Emit("callEnded")
		})

		// Chat Rooms & Typing
		client.On("joinChat", func(args ...any) {
			if len(args) > 0 {
				if chatID, ok := args[0].(string); ok && chatID != "" {
					client.Join(socket.Room(chatID))
				}
			}
		})

		client.On("leaveChat", func(args ...any) {
			if len(args) > 0 {
				if chatID, ok := args[0].(string); ok && chatID != "" {
					client.Leave(socket.Room(chatID))
				}
			}
		})

		client.On("typing", func(args ...any) {
			if len(args) > 0 {
				if room, ok := args[0].(string); ok {
					client.To(socket.Room(room)).Emit("typing", room)
				}
			}
		})

		client.On("stopTyping", func(args ...any) {
			if len(args) > 0 {
				if room, ok := args[0].(string); ok {
					client.To(socket.Room(room)).Emit("stopTyping", room)
				}
			}
		})

		// disconnect handler
		client.On("disconnect", func(...any) {
			Manager.mu.Lock()
			var disconnectedUserID string
			for uid, sid := range Manager.OnlineUsers {
				if sid == socketID {
					disconnectedUserID = uid
					delete(Manager.OnlineUsers, uid)
					break
				}
			}

			delete(Manager.RingingCalls, socketID)

			for roomID, room := range Manager.VideoRooms {
				for uid, p := range room {
					if p.SocketID == string(socketID) {
						delete(room, uid)
						participants := make([]map[string]string, 0, len(room))
						for _, pItem := range room {
							participants = append(participants, map[string]string{
								"id":   pItem.UserID,
								"name": pItem.UserName,
							})
						}
						if len(room) == 0 {
							delete(Manager.VideoRooms, roomID)
						}
						io.To(socket.Room(roomID)).Emit("userLeftRoom", map[string]any{
							"userId":       uid,
							"participants": participants,
						})
					}
				}
			}

			onlineKeys := make([]string, 0, len(Manager.OnlineUsers))
			for k := range Manager.OnlineUsers {
				onlineKeys = append(onlineKeys, k)
			}
			Manager.mu.Unlock()

			if disconnectedUserID != "" {
				io.Emit("onlineUsers", onlineKeys)
			}
		})
	})

	log.Println("[Socket] Socket.IO v4 handler initialized")
	return io, io.ServeHandler(nil)
}

// BroadcastToRoom emits an event to a specific room
func BroadcastToRoom(room string, event string, data any) {
	if Manager != nil && Manager.Server != nil {
		Manager.Server.To(socket.Room(room)).Emit(event, data)
	}
}

// BroadcastGlobal emits an event to all connected clients
func BroadcastGlobal(event string, data any) {
	if Manager != nil && Manager.Server != nil {
		Manager.Server.Emit(event, data)
	}
}
