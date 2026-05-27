package repo

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/Hana-ame/api-pack/shijima/model"
	"github.com/hashicorp/go-multierror"
)

const PageSize = 30

func (r *Repo) MessageCreate(m *model.Message) (int64, error) {
	result, err := r.db.Exec(
		`INSERT INTO message (channel_id, parent_id, title, author_id, author_name, content, image, country, ip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ChannelID, m.ParentID, m.Title, m.Author.ID, m.Author.Name, m.Content, m.Image, m.Country, m.IP,
	)
	if err != nil {
		return -1, fmt.Errorf("insert message: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("get id: %w", err)
	}
	return id, nil
}

func (r *Repo) MessageGet(mid int) (*model.Message, error) {
	var m model.Message
	err := r.db.QueryRow(
		`SELECT id, channel_id, parent_id, title, author_id, author_name, content, image, created_at, edited_at
		FROM message WHERE id = ? AND deleted = 0`, mid,
	).Scan(&m.ID, &m.ChannelID, &m.ParentID, &m.Title, &m.Author.ID, &m.Author.Name, &m.Content, &m.Image, &m.CreatedAt, &m.EditedAt)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	return &m, nil
}

func (r *Repo) MessageList(channelID int, before int, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = PageSize
	}
	cols := `id, channel_id, parent_id, title, author_id, author_name, content, image, created_at, edited_at`
	var rows *sql.Rows
	var err error
	if before > 0 {
		rows, err = r.db.Query(
			`SELECT `+cols+` FROM message WHERE channel_id = ? AND parent_id = 0 AND deleted = 0 AND id < ?
			ORDER BY id DESC LIMIT ?`, channelID, before, limit,
		)
	} else {
		rows, err = r.db.Query(
			`SELECT `+cols+` FROM message WHERE channel_id = ? AND parent_id = 0 AND deleted = 0
			ORDER BY id DESC LIMIT ?`, channelID, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var msgs []*model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.ParentID, &m.Title, &m.Author.ID, &m.Author.Name, &m.Content, &m.Image, &m.CreatedAt, &m.EditedAt); err != nil {
			continue
		}
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

func (r *Repo) MessageReplies(parentID int) ([]*model.Message, error) {
	rows, err := r.db.Query(
		`SELECT id, channel_id, parent_id, title, author_id, author_name, content, image, created_at, edited_at
		FROM message WHERE parent_id = ? AND deleted = 0 ORDER BY id ASC`, parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get replies: %w", err)
	}
	defer rows.Close()

	var msgs []*model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.ParentID, &m.Title, &m.Author.ID, &m.Author.Name, &m.Content, &m.Image, &m.CreatedAt, &m.EditedAt); err != nil {
			continue
		}
		msgs = append(msgs, &m)
	}
	return msgs, rows.Err()
}

func (r *Repo) MessageEdit(mid int, title, content string) error {
	_, err := r.db.Exec(
		`UPDATE message SET title = ?, content = ?, edited_at = datetime('now') WHERE id = ? AND deleted = 0`,
		title, content, mid,
	)
	return err
}

func (r *Repo) MessageDelete(mid int, authorID, ip string) error {
	_, err := r.db.Exec(
		`UPDATE message SET deleted = -1 WHERE id = ? AND (author_id = ? OR ip = ?)`,
		mid, authorID, ip,
	)
	return err
}

func (r *Repo) ThreadWithReplies(tid, pn int) (*model.BoardThread, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var thread *model.Thread
	var replies []*model.Thread
	var err1, err2 error

	go func() {
		defer wg.Done()
		thread, err1 = r.V2GetThread(tid)
	}()
	go func() {
		defer wg.Done()
		replies, err2 = r.V2GetReplies(tid, pn)
	}()
	wg.Wait()

	var merr *multierror.Error
	merr = multierror.Append(merr, err1)
	merr = multierror.Append(merr, err2)
	if err := merr.ErrorOrNil(); err != nil {
		return nil, err
	}

	var num int
	r.db.QueryRow(`SELECT COUNT(*) FROM message WHERE parent_id = ? AND deleted = 0`, tid).Scan(&num)

	return &model.BoardThread{
		Thread:     *thread,
		ReplyCount: -num,
		Replies:    replies,
	}, nil
}
