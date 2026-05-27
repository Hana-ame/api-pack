package repo

import (
	"fmt"

	"github.com/Hana-ame/api-pack/shijima/model"
)

func (r *Repo) ChannelList() ([]model.Channel, error) {
	rows, err := r.db.Query(`SELECT id, name, description, mode, created_at FROM channel ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var chs []model.Channel
	for rows.Next() {
		var c model.Channel
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Mode, &c.CreatedAt); err != nil {
			continue
		}
		chs = append(chs, c)
	}
	return chs, rows.Err()
}

func (r *Repo) ChannelGet(cid int) (*model.Channel, error) {
	var c model.Channel
	err := r.db.QueryRow(
		`SELECT id, name, description, mode, created_at FROM channel WHERE id = ?`, cid,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Mode, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	return &c, nil
}

func (r *Repo) ChannelCreate(name, mode string) (int, error) {
	if mode == "" {
		mode = "chat"
	}
	result, err := r.db.Exec(`INSERT INTO channel (name, mode) VALUES (?, ?)`, name, mode)
	if err != nil {
		return 0, fmt.Errorf("create channel: %w", err)
	}
	id, _ := result.LastInsertId()
	return int(id), nil
}

func (r *Repo) ChannelEnsure(cid int) error {
	_, err := r.db.Exec(`INSERT OR IGNORE INTO channel (id, name, mode) VALUES (?, ?, 'forum')`, cid, fmt.Sprintf("board-%d", cid))
	return err
}
