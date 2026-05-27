package repo

import "github.com/Hana-ame/api-pack/utils/orderedmap"

func (r *Repo) ReactionSet(messageID int, emoji string) error {
	_, err := r.db.Exec(
		`INSERT INTO reaction (message_id, emoji, count, updated_at)
		VALUES (?, ?, 1, datetime('now'))
		ON CONFLICT(message_id, emoji) DO UPDATE SET
			count = count + 1,
			updated_at = datetime('now')`,
		messageID, []byte(emoji),
	)
	return err
}

func (r *Repo) ReactionGet(messageID int) (*orderedmap.OrderedMap, error) {
	rows, err := r.db.Query(
		`SELECT emoji, count FROM reaction WHERE message_id = ? ORDER BY count DESC, updated_at ASC`, messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	om := orderedmap.NewOrderedMap()
	for rows.Next() {
		var emoji string
		var count int
		if err := rows.Scan(&emoji, &count); err != nil {
			return nil, err
		}
		om.Set(emoji, count)
	}
	return om, rows.Err()
}
