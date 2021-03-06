package handler

import (
    "net/http"

    "github.com/matryer/way"
    "github.com/secmohammed/go-twitter/internal/service"
)

type handler struct {
    *service.Service
}

// New  creates an http.Handler with predefined routing.
func New(s *service.Service) http.Handler {
    h := &handler{s}
    api := way.NewRouter()
    api.HandleFunc("POST", "/login", h.login)
    api.HandleFunc("POST", "/send_magic_link", h.sendMagicLink)
    api.HandleFunc("GET", "/auth_redirect", h.authRedirect)
    api.HandleFunc("GET", "/user", h.authUser)
    api.HandleFunc("POST", "/users/:username/toggle_follow", h.toggleFollow)
    api.HandleFunc("PUT", "/user/avatar", h.updateAvatar)
    api.HandleFunc("POST", "/users", h.createUser)
    api.HandleFunc("GET", "/users", h.users)
    api.HandleFunc("GET", "/users/:username", h.user)
    api.HandleFunc("GET", "/users/:username/followers", h.followers)
    api.HandleFunc("GET", "/users/:username/posts", h.posts)
    api.HandleFunc("GET", "/users/:username/followees", h.followees)

    api.HandleFunc("POST", "/posts", h.createPost)
    api.HandleFunc("GET", "/posts/:post_id", h.post)
    api.HandleFunc("POST", "/posts/:post_id/toggle_like", h.togglePostLike)
    api.HandleFunc("POST", "/posts/:post_id/comments", h.createComment)
    api.HandleFunc("GET", "/posts/:post_id/comments", h.comments)

    api.HandleFunc("POST", "/comments/:comment_id/toggle_like", h.toggleCommentLike)
    api.HandleFunc("GET", "/timeline", h.timeline)
    api.HandleFunc("POST", "/posts/:post_id/toggle_subscription", h.togglePostSubscription)

    api.HandleFunc("GET", "/notifications", h.notifications)
    api.HandleFunc("POST", "/notifications/:notification_id/mark_as_read", h.markNotificationAsRead)
    api.HandleFunc("POST", "/mark_notifications_as_read", h.markAllNotificationsAsRead)

    fs := http.FileServer(&spaFileSystem{http.Dir("public")})
    r := way.NewRouter()
    r.Handle("*", "/api...", http.StripPrefix("/api", h.withAuth(api)))
    r.Handle("GET", "/...", fs)
    return r
}
