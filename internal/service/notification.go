package service

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "time"

    "github.com/lib/pq"
)

// Notification model.
type Notification struct {
    ID       int64     `json:"id"`
    UserID   int64     `json:"-"`
    Actors   []string  `json:"actors"`
    Type     string    `json:"type"`
    Read     bool      `json:"read"`
    PostID   *int64    `json:"post_id, omitempty"`
    IssuedAt time.Time `json:"issued_at"`
}
type notificationClient struct {
    notifications chan Notification
    userID        int64
}

// Notifications for the authenticated user in desc order with backward pagination
func (s *Service) Notifications(ctx context.Context, last int, before int64) ([]Notification, error) {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return nil, ErrUnauthenticated
    }
    last = normalizePageSize(last)
    query, args, err := buildQuery(`
        SELECT id, actors, type, read, issued_at, post_id
        FROM notifications
        WHERE user_id = @uid
        {{if .before}}AND id < @before{{end}}
        ORDER BY issued_at DESC
        LIMIT @last
        `, map[string]interface{}{
        "uid":    uid,
        "before": before,
        "last":   last,
    })
    if err != nil {
        return nil, fmt.Errorf("couldn't build notification sql query: %v", err)
    }
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("couldn't query select notifications: %v", err)
    }
    defer rows.Close()
    notifications := make([]Notification, 0, last)
    for rows.Next() {
        var notification Notification
        if err = rows.Scan(&notification.ID, pq.Array(&notification.Actors), &notification.Type, &notification.Read, &notification.IssuedAt, &notification.PostID); err != nil {
            return nil, fmt.Errorf("Couldn't scan notification: %v", err)
        }
        notifications = append(notifications, notification)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("Couldn't iterate over notification rows:%v", err)
    }
    return notifications, nil
}
func (s *Service) MarkNotificationAsRead(ctx context.Context, notificationID int64) error {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return ErrUnauthenticated
    }
    query := "UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2"
    if _, err := s.db.Exec(query, notificationID, uid); err != nil {
        return fmt.Errorf("Couldn't update and mark notification as read: %v", err)
    }
    return nil
}
func (s *Service) MarkNotificationsAsRead(ctx context.Context) error {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return ErrUnauthenticated
    }
    query := "UPDATE notifications SET read = true WHERE user_id = $1"
    if _, err := s.db.Exec(query, uid); err != nil {
        return fmt.Errorf("Couldn't update and mark notification as read: %v", err)
    }
    return nil
}
func (s *Service) notifyFollower(followerID, followeeID int64) {
    tx, err := s.db.Begin()
    if err != nil {
        log.Printf("Couldn't begin tx: %v\n", err)
        return
    }
    defer tx.Rollback()
    var actor string
    query := "SELECT username from users WHERE id = $1"
    if err = tx.QueryRow(query, followerID).Scan(&actor); err != nil {
        log.Printf("couldn't query select follow notification actor: %v\n", err)
        return
    }
    var notified bool
    query = `SELECT EXISTS (
        SELECT 1 FROM notifications
        WHERE user_id = $1
            AND actors @> ARRAY[$2]::varchar[]
            AND type = 'follow'
    )`
    if err = tx.QueryRow(query, followeeID, actor).Scan(&notified); err != nil {
        log.Printf("couldn't query select follow notification existence: %v\n", err)
        return
    }
    if notified {
        return
    }
    var notificationID int64
    query = "SELECT id from notifications WHERE user_id = $1 AND type = 'follow' AND read = false"
    err = tx.QueryRow(query, followeeID).Scan(&notificationID)
    if err != nil && err != sql.ErrNoRows {
        log.Printf("couldn't query select unread follow notification: %v\n", err)
        return

    }
    var notification Notification
    if err == sql.ErrNoRows {
        actors := []string{actor}
        query = "INSERT INTO notifications (user_id, actors, type) VALUES ($1, $2, 'follow') RETURNING id, issued_at"
        if err = tx.QueryRow(query, followeeID, pq.Array(actors)).Scan(&notification.ID, &notification.IssuedAt); err != nil {
            log.Printf("Couldn't insert follow notification: %v\n", err)
            return
        }
        notification.Actors = actors
    } else {
        query = `
            UPDATE notifications SET actors = array_prepend($1, notifications.actors), issued_at = now() where id = $2 RETURNING actors, issued_at
        `
        if err = tx.QueryRow(query, actor, notificationID).Scan(pq.Array(&notification.Actors), &notification.IssuedAt); err != nil {
            log.Printf("Couldn't update  follow notification: %v\n", err)
            return
        }
        notification.ID = notificationID
    }
    notification.UserID = followeeID
    notification.Type = "follow"
    if err = tx.Commit(); err != nil {
        log.Printf("Couldn't commit notification: %v\n", err)
        return
    }
    go s.broadcastNotification(notification)
}
func (s *Service) notifyComment(c Comment) {
    actor := c.User.Username

    rows, err := s.db.Query(`
        INSERT INTO notifications (user_id, actors, type, post_id)
        SELECT user_id, $1, 'comment', $2 FROM post_subscriptions
        WHERE post_subscriptions.user_id != $3
            AND post_subscriptions.post_id = $2
        ON CONFLICT (user_id, type, post_id, read) DO UPDATE SET
            actors = array_prepend($4, array_remove(notifications.actors, $4)),
            issued_at = now()
        RETURNING id, user_id, actors, issued_at`,
        pq.Array([]string{actor}),
        c.PostID,
        c.UserID,
        actor,
    )
    if err != nil {
        log.Printf("couldn't insert notification with comment: %v", err)
        return
    }
    defer rows.Close()
    for rows.Next() {
        var notification Notification
        if err = rows.Scan(&notification.ID, &notification.UserID, pq.Array(&notification.Actors), &notification.IssuedAt); err != nil {
            log.Printf("Couldn't scan comment notification: %v", err)
            return
        }
        notification.Type = "comment"
        notification.PostID = &c.PostID
        go s.broadcastNotification(notification)
    }
    if err = rows.Err(); err != nil {
        log.Printf("Couldn't iterate over comment notification rows: %v\n", err)
        return
    }
}
func (s *Service) notifyPostMention(p Post) {
    mentions := collectMentions(p.Content)
    if len(mentions) == 0 {
        return
    }
    actors := []string{p.User.Username}
    rows, err := s.db.Query(`
        INSERT INTO notifications (user_id, actors, type, post_id)
        SELECT users.id, $1, 'post_mention', $2 FROM users
        WHERE users.id != $3 AND username = ANY($4)
        RETURNING id, user_id, issued_at`,
        pq.Array(actors),
        p.ID,
        p.UserID,
        pq.Array(mentions),
    )
    if err != nil {
        log.Printf("Couldn't insert post mention notification: %v", err)
        return
    }
    defer rows.Close()
    for rows.Next() {
        var n Notification
        if err = rows.Scan(&n.ID, &n.UserID, &n.IssuedAt); err != nil {
            log.Printf("Couldn't scan post mention notification: %v\n", err)
            return
        }
        n.Actors = actors
        n.Type = "post_mention"
        n.PostID = &p.ID
        go s.broadcastNotification(n)

    }
    if err = rows.Err(); err != nil {
        log.Printf("Couldn't iterate over post mention notification rows: %v\n", err)
        return
    }
}

func (s *Service) SubscribeToNotifications(ctx context.Context) (chan Notification, error) {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return nil, ErrUnauthenticated
    }
    nn := make(chan Notification)
    c := &notificationClient{notifications: nn, userID: uid}
    s.notificationClients.Store(c, struct{}{})
    go func() {
        <-ctx.Done()
        s.notificationClients.Delete(c)
        close(nn)
    }()
    return nn, nil
}
func (s *Service) broadcastNotification(n Notification) {
    s.notificationClients.Range(func(key, _ interface{}) bool {
        client := key.(*notificationClient)
        if client.userID == n.UserID {
            client.notifications <- n
        }
        return true
    })
}
func (s *Service) notifyCommentMention(c Comment) {
    mentions := collectMentions(c.Content)
    if len(mentions) == 0 {
        return
    }
    actor := c.User.Username
    rows, err := s.db.Query(`
        INSERT INTO notifications (user_id, actors, type, post_id)
        SELECT users.id, $1, 'comment_mention', $2 FROM users
        WHERE users.id != $3 AND username = ANY($4)
        ON CONFLICT (user_id, type, post_id, read) DO UPDATE SET
            actors = array_prepend($5, array_remove(notifications.actors, $5)),
            issued_at = now()
        RETURNING id, user_id, issued_at`,
        pq.Array([]string{actor}),
        c.PostID,
        c.UserID,
        pq.Array(mentions),
        actor,
    )
    if err != nil {
        log.Printf("Couldn't insert comment mention notification: %v", err)
        return
    }
    defer rows.Close()
    for rows.Next() {
        var n Notification
        if err = rows.Scan(&n.ID, &n.UserID, pq.Array(&n.Actors), &n.IssuedAt); err != nil {
            log.Printf("Couldn't scan comment mention notification: %v\n", err)
            return
        }
        n.Type = "comment_mention"
        n.PostID = &c.PostID
        go s.broadcastNotification(n)
    }
    if err = rows.Err(); err != nil {
        log.Printf("Couldn't iterate over comment mention notification rows: %v\n", err)
        return
    }
}
