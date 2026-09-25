# Aurora Workspace - Go Backend

A high-performance, concurrent backend rewritten in pure Go (`net/http`) while preserving 100% API and Socket.IO compatibility with the Aurora Workspace frontend.

## 🚀 Key Improvements in Go

1. **Pure Go (`net/http`)**: Zero unnecessary heavy frameworks; standard library routing with Go 1.22+ method & path parameter matching (`GET /api/tasks/{id}`).
2. **Native PDF Preview & Streaming**: Fixed Cloudinary limitations by implementing an inline file streaming engine with `Content-Disposition: inline`, `Content-Type: application/pdf`, and HTTP byte-range support (`Accept-Ranges: bytes`) for smooth browser PDF viewing.
3. **Socket.IO v4 Compatibility**: Built using `github.com/zishang520/socket.io/v2` to maintain seamless real-time chat, video signaling (WebRTC), and dashboard notifications without requiring frontend rewrites.
4. **Lightweight & Fast**: Compiled single binary starting up in milliseconds and consuming only ~15–25 MB RAM.
5. **Compile-Time Safety**: Strict MongoDB BSON structs and models preventing runtime null-pointer and type-mismatch bugs.

---

## 📁 Project Structure

```
backend/
├── main.go                     # Server bootstrap, cron jobs, graceful shutdown
├── go.mod / go.sum             # Go module dependencies
├── config/
│   ├── db.go                   # MongoDB official Go driver connection
│   └── storage.go              # Storage engine with inline PDF viewer & byte-range streaming
├── models/
│   ├── user.go                 # User model, bcrypt hashing & verification
│   ├── team.go                 # Team model & populated responses
│   ├── task.go                 # Kanban Task model
│   ├── document.go             # Document metadata & folder hierarchy
│   ├── folder.go               # Folder structure model
│   ├── message.go              # Direct & group chat message model
│   ├── conversation.go         # Chat room conversation model
│   ├── event.go                # Calendar event model
│   ├── notification.go         # Notification model
│   ├── password.go             # Password manager model
│   ├── activity.go             # Team activity feed model
│   └── audit_log.go            # System admin audit logs
├── middleware/
│   ├── auth.go                 # JWT authentication, user status, role protection
│   ├── cors.go                 # Dynamic CORS allowing frontend & Vercel
│   └── ratelimit.go            # Sliding window in-memory rate limiter
├── controllers/
│   ├── auth_controller.go      # Login, registration, profile, search
│   ├── task_controller.go      # Task CRUD & team notifications
│   ├── team_controller.go      # Team management & member statistics
│   ├── document_controller.go  # File uploads, folders, and inline PDF view
│   ├── password_controller.go  # Password manager CRUD
│   ├── chat_controller.go      # Conversations, messages, attachments
│   ├── event_controller.go     # Calendar events & reminders
│   ├── notification_controller.go # Notification status sync & counts
│   └── admin/
│       ├── system_admin_controller.go # Platform metrics, audit logs, broadcasts
│       ├── team_admin_controller.go   # Global team administration
│       └── user_admin_controller.go   # User approvals, roles, impersonation
├── routes/
│   └── routes.go               # Route registration mapping all endpoints
├── socket/
│   └── socket_handler.go       # WebRTC signaling, chat, presence, notifications
└── utils/
    ├── token.go                # JWT token signing & verification
    ├── response.go             # Standard HTTP JSON writer
    ├── logger.go               # Conditional debug logger
    └── notification.go         # Bulk notification & team update dispatchers
```

---

## 🛠️ How to Run

### 1. Set Environment Variables
Copy `.env.example` to `.env` inside `backend/`:
```bash
cp .env.example .env
```
Ensure `MONGO_URI` and `JWT_SECRET` are set.

### 2. Run Directly
```powershell
$env:PATH = "D:\Golang\bin;" + $env:PATH
go run main.go
```

### 3. Build Production Executable
```powershell
$env:PATH = "D:\Golang\bin;" + $env:PATH
go build -o backend.exe main.go
./backend.exe
```
