package handler

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"

    "github.com/matryer/way"
    "github.com/secmohammed/go-twitter/internal/service"
)

type createUserInput struct {
    Email, Username string
}

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
    var createUserInput createUserInput
    defer r.Body.Close()
    if err := json.NewDecoder(r.Body).Decode(&createUserInput); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return

    }
    err := h.CreateUser(r.Context(), createUserInput.Email, createUserInput.Username)
    if err == service.ErrInvalidEmail || err == service.ErrInvalidUsername {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err == service.ErrEmailNotUnique || err == service.ErrUsernameNotUnique {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
func (h *handler) user(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    username := way.Param(ctx, "username")
    u, err := h.User(ctx, username)
    if err == service.ErrInvalidUsername {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err == service.ErrUserNotFound {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, u, http.StatusOK)
}
func (h *handler) users(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    search := q.Get("search")
    first, _ := strconv.Atoi(q.Get("first"))
    after := q.Get("after")
    response, err := h.Users(r.Context(), search, first, after)
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, response, http.StatusOK)
}
func (h *handler) followers(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q := r.URL.Query()
    username := way.Param(ctx, "username")
    first, _ := strconv.Atoi(q.Get("first"))
    after := q.Get("after")
    response, err := h.Followers(ctx, username, first, after)
    if err == service.ErrInvalidUsername {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, response, http.StatusOK)
}
func (h *handler) updateAvatar(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, service.MaxAvatarBytes)
    defer r.Body.Close()
    avatarURL, err := h.UpdateAvatar(r.Context(), r.Body)
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err == service.ErrUnsupportedAvatarFormat {
        http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    fmt.Fprint(w, avatarURL)
}
func (h *handler) followees(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    q := r.URL.Query()
    username := way.Param(ctx, "username")
    first, _ := strconv.Atoi(q.Get("first"))
    after := q.Get("after")
    response, err := h.Followees(ctx, username, first, after)
    if err == service.ErrInvalidUsername {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }

    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, response, http.StatusOK)
}

func (h *handler) toggleFollow(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    username := way.Param(ctx, "username")
    response, err := h.ToggleFollow(ctx, username)
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err == service.ErrInvalidUsername {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err == service.ErrUserNotFound {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    if err == service.ErrForbiddenFollow {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, response, http.StatusOK)
}
