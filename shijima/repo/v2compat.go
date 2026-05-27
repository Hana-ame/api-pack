package repo

import (
	"fmt"
	"sync"

	"github.com/Hana-ame/api-pack/shijima/model"
)

func (r *Repo) V2GetThread(no int) (*model.Thread, error) {
	var t model.Thread
	var parentID int
	err := r.db.QueryRow(
		`SELECT id, author_name, author_id, created_at, image, title, content, parent_id
		FROM message WHERE id = ? AND deleted = 0`, no,
	).Scan(&t.No, &t.Name, &t.ID, &t.Created, &t.Image, &t.Title, &t.Content, &parentID)
	if err != nil {
		return nil, fmt.Errorf("thread not found: %w", err)
	}
	t.ReplyTo = uint(parentID)
	return &t, nil
}

func (r *Repo) V2GetReplies(no, pn int) ([]*model.Thread, error) {
	offset := pn * model.PageSize
	rows, err := r.db.Query(
		`SELECT id, author_name, author_id, created_at, image, content
		FROM message WHERE parent_id = ? AND deleted = 0 ORDER BY id ASC LIMIT ? OFFSET ?`,
		no, model.PageSize, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get replies: %w", err)
	}
	defer rows.Close()

	var replies []*model.Thread
	for rows.Next() {
		var t model.Thread
		if err := rows.Scan(&t.No, &t.Name, &t.ID, &t.Created, &t.Image, &t.Content); err != nil {
			continue
		}
		replies = append(replies, &t)
	}
	return replies, rows.Err()
}

func (r *Repo) V2GetRepliesPreview(no int) ([]*model.Thread, error) {
	rows, err := r.db.Query(
		`SELECT id, author_name, author_id, created_at, image, content
		FROM message WHERE parent_id = ? AND deleted = 0 ORDER BY id DESC LIMIT 5`,
		no,
	)
	if err != nil {
		return nil, fmt.Errorf("get replies preview: %w", err)
	}
	defer rows.Close()

	var replies []*model.Thread
	for rows.Next() {
		var t model.Thread
		if err := rows.Scan(&t.No, &t.Name, &t.ID, &t.Created, &t.Image, &t.Content); err != nil {
			continue
		}
		replies = append(replies, &t)
	}
	for i, j := 0, len(replies)-1; i < j; i, j = i+1, j-1 {
		replies[i], replies[j] = replies[j], replies[i]
	}
	return replies, rows.Err()
}

func (r *Repo) V2GetBoardThreads(bid, pn int) ([]*model.BoardThread, error) {
	half := model.PageSize / 2
	offset := half * pn
	rows, err := r.db.Query(
		`SELECT m.id, m.author_name, m.author_id, m.created_at, m.image, m.title, m.content,
			(SELECT COUNT(*) FROM message WHERE parent_id = m.id AND deleted = 0) AS replynum
		FROM message m
		WHERE m.channel_id = ? AND m.parent_id = 0 AND m.deleted = 0
		ORDER BY m.id DESC LIMIT ? OFFSET ?`,
		bid, half, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get board threads: %w", err)
	}
	defer rows.Close()

	var threads []*model.BoardThread
	for rows.Next() {
		var bt model.BoardThread
		if err := rows.Scan(&bt.No, &bt.Name, &bt.ID, &bt.Created, &bt.Image, &bt.Title, &bt.Content, &bt.ReplyCount); err != nil {
			continue
		}
		threads = append(threads, &bt)
	}
	return threads, rows.Err()
}

func (r *Repo) V2GetBoard(bid, pn int) ([]*model.BoardThread, error) {
	threads, err := r.V2GetBoardThreads(bid, pn)
	if err != nil {
		return nil, err
	}

	n := len(threads)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			replies, e := r.V2GetRepliesPreview(int(threads[i].No))
			if e == nil {
				threads[i].Replies = replies
			}
			errs[i] = e
		}(i)
	}
	wg.Wait()
	return threads, nil
}

func (r *Repo) V2PostThread(t *model.Thread, bid int) (int64, error) {
	r.db.Exec(`INSERT OR IGNORE INTO channel (id, name, mode) VALUES (?, ?, 'forum')`, bid, fmt.Sprintf("board-%d", bid))

	result, err := r.db.Exec(
		`INSERT INTO message (channel_id, parent_id, title, author_id, author_name, content, image, country, ip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bid, t.ReplyTo, t.Title, t.ID, t.Name, t.Content, t.Image, t.Country, t.IP,
	)
	if err != nil {
		return -1, fmt.Errorf("post thread: %w", err)
	}
	id, _ := result.LastInsertId()
	t.No = uint(id)
	return id, nil
}

func (r *Repo) V2DeleteThread(no int, id, ip string) error {
	_, err := r.db.Exec(
		`UPDATE message SET deleted = -1 WHERE id = ? AND (author_id = ? OR ip = ?)`,
		no, id, ip,
	)
	return err
}

func (r *Repo) V2GetBoardIDs() ([]int, error) {
	rows, err := r.db.Query(`SELECT DISTINCT id FROM channel ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bids []int
	for rows.Next() {
		var bid int
		if err := rows.Scan(&bid); err != nil {
			continue
		}
		bids = append(bids, bid)
	}
	return bids, nil
}
