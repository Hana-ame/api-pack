package pastejson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	_ "github.com/Hana-ame/api-pack/tools/utils"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var connStr = os.Getenv("PASTEJSON_CONN_STR")

type DBPool struct {
	*pgxpool.Pool
}

func NewDBPool(ctx context.Context, connStr string) (*DBPool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dbpool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, err
	}
	return &DBPool{dbpool}, nil
}
func (p *DBPool) Close() {
	p.Pool.Close() // 关闭连接池，释放所有连接
}

// TableExistsWithRegclass 使用 to_regclass 函数检查表是否存在（PostgreSQL 9.4+）
// 注意：此方法通常用于检查当前搜索路径下的表，或包含模式名的完整标识
func (p *DBPool) TableExistsWithRegclass(ctx context.Context, tableIdentifier string) (bool, error) {
	query := `SELECT to_regclass($1) IS NOT NULL`
	var exists bool
	err := p.QueryRow(ctx, query, tableIdentifier).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence with to_regclass: %w", err)
	}
	return exists, nil
}

// CreateTable 执行创建表的SQL语句。
// 参数:
// - ctx: 用于控制查询的上下文，如设置超时或取消信号。
// - createTableSQL: 包含完整CREATE TABLE语句的字符串。建议包含`IF NOT EXISTS`子句以避免重复创建错误。
// - ignoreExists: 如果为true，则在创建语句中自动添加`IF NOT EXISTS`子句（如果原语句中不含该子句）。
// 返回值:
// - created: 表是否被成功创建（如果表已存在且未发生错误，则返回false）。
// - err: 执行过程中发生的任何错误。
func (p *DBPool) CreateTable(ctx context.Context, createTableSQL string) (bool, error) {
	sqlToExecute := createTableSQL

	// 使用Exec执行不返回数据行的SQL命令
	result, err := p.Exec(ctx, sqlToExecute)
	if err != nil {
		// 检查错误是否为“表已存在”的错误（PostgreSQL错误码42P07）
		if isTableExistsError(err) {
			// 表已经存在，这不是一个致命的错误，但我们返回created=false
			return false, nil
		}
		// 对于其他错误，返回错误信息
		return false, fmt.Errorf("failed to create table: %w", err)
	}

	// 虽然CREATE TABLE不返回行影响计数，但成功执行后result不为nil
	// 如果执行到这里，通常意味着表被创建了
	_ = result // 可以选择忽略result，因为CREATE TABLE不返回标准的行计数
	return true, nil
}

// ExecuteInTransaction 在事务中执行一系列数据库操作。
// 参数:
//   - ctx: 上下文，用于控制事务的超时和取消。
//   - txFunc: 一个在事务中执行的函数。该函数接收一个事务上下文和一个 pgx.Tx 对象。
//     如果此函数返回错误，事务将被回滚；如果返回 nil，事务将被提交。
//
// 返回值:
//   - err: 执行过程中发生的任何错误。包括事务开始、txFunc 执行以及提交/回滚过程中的错误。
func (p *DBPool) ExecuteInTransaction(ctx context.Context, txOptions pgx.TxOptions, txFunc func(context.Context, pgx.Tx) error) error {
	// 1. 从事务池中开启一个事务。
	//    使用 pgx.TxOptions 可以设置事务的隔离级别和访问模式（读/写）。
	//    如果使用 nil，则采用数据库的默认设置。
	tx, err := p.BeginTx(ctx, txOptions)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// 使用 defer 确保在函数返回前，如果事务还未结束（提交或回滚），则进行回滚。
	// 这是一种安全措施，防止因函数中途返回或发生 panic 而导致事务悬挂。
	defer func() {
		if tx != nil {
			// 如果 tx 不为 nil，说明事务还没有被显式地提交或回滚。
			// 此时回滚事务，并忽略回滚操作可能产生的错误（因为我们更关心主操作的错误）。
			_ = tx.Rollback(ctx)
		}
	}()

	// 2. 执行用户传入的事务操作函数。
	err = txFunc(ctx, tx)
	if err != nil {
		// 如果用户函数执行出错，尝试回滚事务。
		// 回滚错误和业务错误都需要返回，但业务错误是根本原因。
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("txFunc failed: %v, and rollback also failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("txFunc failed, transaction rolled back: %w", err)
	}

	// 3. 如果用户函数执行成功，提交事务。
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 4. 将 tx 设置为 nil，这样 defer 函数中的 Rollback 就不会再执行。
	tx = nil
	return nil
}

// =======================================================
// =======================================================
// =======================================================
// =======================================================
// =======================================================
// =======================================================

// isTableExistsError 检查错误是否表示表已存在（PostgreSQL错误码42P07）
func isTableExistsError(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "42P07" // duplicate_table错误码
	}
	return false
}

// AddJsonData 向 json_data 表插入一条新的 JSONB 数据记录。
// 参数:
//   - ctx: 上下文，用于控制查询的超时和取消。
//   - tx: 正在进行中的数据库事务。
//   - id: 要插入记录的主键 ID。调用者需确保其唯一性。
//   - data: 要存储的 JSONB 数据，类型为 []byte。应确保是有效的 JSON。
//
// 返回值:
//   - error: 如果插入过程中发生错误（如违反主键约束、JSON 格式无效等），则返回错误。
func AddJsonData(ctx context.Context, tx pgx.Tx, id int64, data []byte) error {
	// SQL 插入语句。使用 $1, $2 作为参数占位符。
	const sql = `INSERT INTO json_data (id, data) VALUES ($1, $2)`

	// 使用事务的 Exec 方法执行插入操作。
	_, err := tx.Exec(ctx, sql, id, data)
	if err != nil {
		// 包装错误并返回，提供更多上下文信息。
		return fmt.Errorf("failed to insert into json_data with id %d: %w", id, err)
	}

	// 插入成功，返回 nil。
	return nil
}

// AddTag 向 json_tags 表插入一条标签记录，关联到指定的 json_data id。
// 参数:
//   - ctx: 上下文。
//   - tx: 数据库事务。
//   - id: json_data 表的主键 ID，此记录必须已存在。
//   - tag: 要添加的标签字符串。
//
// 返回值:
//   - error: 如果插入失败（如违反外键约束或联合主键约束），则返回错误。
func AddTag(ctx context.Context, tx pgx.Tx, id int64, tag string) error {
	if tag == "" {
		return fmt.Errorf("tag is empty")
	}
	const sql = `INSERT INTO json_tags (id, tag) VALUES ($1, $2)`

	_, err := tx.Exec(ctx, sql, id, tag)
	if err != nil {
		return fmt.Errorf("failed to insert tag '%s' for id %d: %w", tag, id, err)
	}

	return nil
}

// AddPrevious 向 previous_relations 表插入一条关系记录，表示 previous_id 是 id 的“前一个”。
// 参数:
//   - ctx: 上下文。
//   - tx: 数据库事务。
//   - previousID: 作为“前一个”的记录的 ID。
//   - id: 当前记录的 ID。
//
// 返回值:
//   - error: 如果插入失败（如违反外键约束或联合主键约束），则返回错误。
func AddPrevious(ctx context.Context, tx pgx.Tx, previousID int64, id int64) error {
	const sql = `INSERT INTO previous_relations (previous_id, id) VALUES ($1, $2)`

	_, err := tx.Exec(ctx, sql, previousID, id)
	if err != nil {
		return fmt.Errorf("failed to create relation from previous_id %d to id %d: %w", previousID, id, err)
	}

	return nil
}

// GetRecordByID 根据 id 从 json_data 表查询记录，并直接返回 data 字段的 []byte 值。
// 参数:
//   - ctx: 上下文，用于控制查询的超时和取消。
//   - tx: 正在进行中的数据库事务。
//   - id: 要查询的记录的主键 ID。
//
// 返回值:
//   - []byte: 查询到的 JSONB 数据对应的字节切片。如果记录不存在或 data 字段为 NULL，则返回 nil。
//   - error: 执行过程中发生的任何错误（如查询失败、连接问题等）。
func GetRecordByID(ctx context.Context, tx pgx.Tx, id int64) ([]byte, error) {
	// SQL 查询语句，只选择 data 字段
	query := `SELECT data FROM json_data WHERE id = $1`

	var data []byte
	// 使用 QueryRow 查询单行，并 Scan 到 data 变量中
	err := tx.QueryRow(ctx, query, id).Scan(&data)

	if err != nil {
		// 判断错误是否为 pgx.ErrNoRows（即没有找到记录）
		if errors.Is(err, pgx.ErrNoRows) {
			// 没有找到记录，返回 nil 数据和 nil 错误（或者可以返回一个自定义错误，根据业务逻辑决定）
			return nil, nil
		}
		// 对于其他错误，返回错误信息
		return nil, fmt.Errorf("failed to query json_data for id %d: %w", id, err)
	}

	// 成功查询到数据，返回字节切片
	return data, nil
}

// PaginatedResponse 是用于分页响应的结构体
type PaginatedResponse struct {
	Query    string                     `json:"query"`              // 当前查询的路径和参数
	Previous string                     `json:"previous,omitempty"` // 上一页的链接，若无则省略
	Next     string                     `json:"next,omitempty"`     // 下一页的链接，若无则省略
	Payload  map[string]json.RawMessage `json:"payload,omitempty"`  // 存储ID与原始JSON数据的映射，若无则省略
}

// isValidTableName 验证表名是否只包含字母、数字和下划线，以防止SQL注入
// func isValidTableName(tag string) bool {
// 	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, tag)
// 	return matched
// }

// GetRecords 查询指定ID列表的记录，返回一个map，其中键为ID的字符串形式，值为原始的JSON数据。
// 参数:
//   - ctx: 上下文，用于控制查询的超时和取消。
//   - ids: 要查询的记录ID列表。
//
// 返回值:
//   - map[string]json.RawMessage: 键值对映射，键是ID的字符串形式，值是原始的JSON数据。如果某些ID不存在，它们不会出现在map中。
//   - error: 执行过程中发生的任何错误（如查询失败、连接问题等）。
func GetRecords(ctx context.Context, tx pgx.Tx, ids []int64) (map[string]json.RawMessage, error) {
	// 处理空ID列表的情况，直接返回空map
	if len(ids) == 0 {
		return make(map[string]json.RawMessage), nil
	}

	// 使用 PostgreSQL 的 ANY 子句进行批量查询，避免SQL注入风险
	query := `SELECT id, data FROM json_data WHERE id = ANY($1)`
	rows, err := tx.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to query records for IDs %v: %w", ids, err)
	}
	defer rows.Close()

	// 初始化结果map
	result := make(map[string]json.RawMessage)
	for rows.Next() {
		var id int64
		var data json.RawMessage
		// 扫描每一行的id和data字段
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		// 将int64类型的ID转换为字符串形式作为map的键
		key := strconv.FormatInt(id, 10)
		result[key] = data
	}

	// 检查迭代过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return result, nil
}

func GetRecordsByTag(ctx context.Context, tx pgx.Tx, tag string, pageSize, lessThan, greaterThan int) (map[string]json.RawMessage, error) {
	// 验证表名是否合法，防止SQL注入
	// 傻逼ai
	// if !isValidTableName(tag) {
	// 	return nil, fmt.Errorf("invalid tag name: %s", tag)
	// }

	// 杂技.下次还是打表.
	// 有lessthan, 没有greaterthan 使用lessthan, DESC
	// 没有lessthan, 有greaterthan. 使用greaterthan, ASC
	// 两个都没有, 会落在id>0上面, 并且使用DESC ,符合要求
	// 两个都有(状况外), 使用less界, 并且ASC, 是最开始的limit条
	query := (`
		SELECT jd.id, jd.data
		FROM json_data jd
		JOIN json_tags jt ON jd.id = jt.id
		WHERE jt.tag = $1
		` + tools.Ternary(lessThan > 0, `AND jd.id < $2`, `AND jd.id > $2`) + ` 
		ORDER BY jt.id` + tools.Ternary(greaterThan > 0, " ASC ", " DESC ") + `
		LIMIT $3
	`)

	rows, err := tx.Query(ctx, query, tag, tools.Ternary(lessThan > 0, lessThan, greaterThan), pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query records for tag %s: %w", tag, err)
	}
	defer rows.Close()

	result := make(map[string]json.RawMessage)
	for rows.Next() {
		var id string
		var data json.RawMessage
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		// key := strconv.FormatInt(id, 10)
		result[id] = data
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return result, nil
}
