package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

// vecTableNeedsUpgrade reports whether a vec0 virtual table predates the
// current schema. Target schema v2 requires both a user_id partition key
// (KNN scan pruning) and a binary-quantized summary column (cheap recall).
func vecTableNeedsUpgrade(db *sql.DB, tableName string) bool {
	var createSQL string
	err := db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&createSQL)
	if err != nil {
		// Table missing entirely — initVecSchema creates it in the target format.
		return false
	}
	lower := strings.ToLower(createSQL)
	return !strings.Contains(lower, "partition key") ||
		!strings.Contains(lower, "summary_embedding_bin")
}

// migrateVecTablesToLatest rebuilds the embedding vec0 tables to the target
// schema (user_id partition key + summary_embedding_bin bit[1024]):
//   - the partition key constrains KNN brute-force scans to one user
//   - the bit column enables 32x cheaper first-stage recall (Hamming), with
//     callers reranking the recalled rows against the float32 embeddings
//
// vec0 virtual tables cannot be ALTERed or RENAMEd (sqlite-vec issue #43), so
// the rebuild goes through a temporary table:
//  1. create <table>_migr with the target schema
//  2. copy rows backfilled with user_id from the parent table, deriving the
//     binary column via vec_quantize_binary() in SQL
//  3. drop the old table and recreate it under the original name
//  4. copy rows back from the migration table and drop it
//
// Orphaned embedding rows (whose article/cluster no longer exists) are
// discarded by the JOIN — they can never match a user query anyway.
// The migration is idempotent: target-format tables are detected and skipped.
func migrateVecTablesToLatest(db *sql.DB) error {
	if err := migrateVecTableToLatest(db,
		"article_embeddings", "article_id", "articles", "id",
	); err != nil {
		return err
	}
	return migrateVecTableToLatest(db,
		"cluster_embeddings", "cluster_id", "clusters", "id",
	)
}

func migrateVecTableToLatest(db *sql.DB, table, pkColumn, parentTable, parentPK string) error {
	if !vecTableNeedsUpgrade(db, table) {
		return nil
	}

	migrTable := table + "_migr"

	if _, err := db.Exec(fmt.Sprintf(
		`CREATE VIRTUAL TABLE %s USING vec0(
			%s INTEGER PRIMARY KEY,
			user_id INTEGER partition key,
			title_embedding float[1024],
			summary_embedding float[1024],
			summary_embedding_bin bit[1024]
		)`, migrTable, pkColumn)); err != nil {
		return fmt.Errorf("create %s migration table: %w", table, err)
	}

	if _, err := db.Exec(fmt.Sprintf(
		`INSERT INTO %s (%s, user_id, title_embedding, summary_embedding, summary_embedding_bin)
		 SELECT e.%s, p.user_id, e.title_embedding, e.summary_embedding,
		        vec_quantize_binary(e.summary_embedding)
		 FROM %s e JOIN %s p ON e.%s = p.%s`,
		migrTable, pkColumn,
		pkColumn,
		table, parentTable, pkColumn, parentPK,
	)); err != nil {
		_, _ = db.Exec(`DROP TABLE ` + migrTable)
		return fmt.Errorf("backfill %s into migration table: %w", table, err)
	}

	if _, err := db.Exec(`DROP TABLE ` + table); err != nil {
		_, _ = db.Exec(`DROP TABLE ` + migrTable)
		return fmt.Errorf("drop legacy %s table: %w", table, err)
	}

	if _, err := db.Exec(fmt.Sprintf(
		`CREATE VIRTUAL TABLE %s USING vec0(
			%s INTEGER PRIMARY KEY,
			user_id INTEGER partition key,
			title_embedding float[1024],
			summary_embedding float[1024],
			summary_embedding_bin bit[1024]
		)`, table, pkColumn)); err != nil {
		return fmt.Errorf("recreate %s with target schema: %w", table, err)
	}

	if _, err := db.Exec(fmt.Sprintf(
		`INSERT INTO %s (%s, user_id, title_embedding, summary_embedding, summary_embedding_bin)
		 SELECT %s, user_id, title_embedding, summary_embedding, summary_embedding_bin FROM %s`,
		table, pkColumn,
		pkColumn, migrTable,
	)); err != nil {
		return fmt.Errorf("restore %s rows from migration table: %w", table, err)
	}

	if _, err := db.Exec(`DROP TABLE ` + migrTable); err != nil {
		return fmt.Errorf("drop %s migration table: %w", table, err)
	}

	return nil
}
