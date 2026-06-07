package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"MavenRSS/internal/models"
)

// CreateCluster creates a new article cluster and returns its ID.
func (db *DB) CreateCluster(userID int64, status string) (int64, error) {
	db.WaitForReady()
	result, err := db.Exec(
		`INSERT INTO clusters (user_id, status) VALUES (?, ?)`,
		userID, status,
	)
	if err != nil {
		return 0, fmt.Errorf("create cluster: %w", err)
	}
	return result.LastInsertId()
}

// CreateStandaloneClusterForArticle creates a cluster and assigns the article in one write transaction.
func (db *DB) CreateStandaloneClusterForArticle(userID, articleID int64, articleIsFavorite bool) (int64, error) {
	db.WaitForReady()
	now := time.Now()
	var clusterID int64

	err := db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		result, err := tx.Exec(
			`INSERT INTO clusters (user_id, status, is_favorite, updated_at) VALUES (?, 'pending_merge', ?, ?)`,
			userID, articleIsFavorite, now,
		)
		if err != nil {
			return fmt.Errorf("create cluster: %w", err)
		}
		clusterID, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get cluster id: %w", err)
		}
		if _, err := tx.Exec(`UPDATE articles SET cluster_id = ? WHERE id = ?`, clusterID, articleID); err != nil {
			return fmt.Errorf("assign article cluster: %w", err)
		}
		if _, err := tx.Exec(
			`UPDATE clusters SET article_count = (SELECT COUNT(*) FROM articles WHERE cluster_id = ?), updated_at = ? WHERE id = ?`,
			clusterID, now, clusterID,
		); err != nil {
			return fmt.Errorf("update cluster article count: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return clusterID, nil
}

// JoinArticleCluster assigns an article to an existing cluster and refreshes cluster metadata atomically.
func (db *DB) JoinArticleCluster(articleID, clusterID int64, articleIsFavorite bool) error {
	db.WaitForReady()
	now := time.Now()

	return db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE articles SET cluster_id = ? WHERE id = ?`, clusterID, articleID); err != nil {
			return fmt.Errorf("assign article cluster: %w", err)
		}
		if articleIsFavorite {
			if _, err := tx.Exec(`UPDATE clusters SET is_favorite = 1, updated_at = ? WHERE id = ?`, now, clusterID); err != nil {
				return fmt.Errorf("sync cluster favorite: %w", err)
			}
		}
		if _, err := tx.Exec(
			`UPDATE clusters SET article_count = (SELECT COUNT(*) FROM articles WHERE cluster_id = ?), status = 'pending_merge', updated_at = ? WHERE id = ?`,
			clusterID, now, clusterID,
		); err != nil {
			return fmt.Errorf("update cluster article count and status: %w", err)
		}
		return nil
	})
}

// GetClusterByID retrieves a cluster by its ID.
func (db *DB) GetClusterByID(clusterID int64) (*models.Cluster, error) {
	db.WaitForReady()
	var c models.Cluster
	err := db.QueryRow(`
		SELECT id, user_id, status, merged_title, merged_summary, merged_content,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, created_at, updated_at, is_read, is_favorite, is_read_later, is_hidden
FROM clusters WHERE id = ?
`, clusterID).Scan(
		&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary, &c.MergedContent,
		&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
		&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := db.applyClusterMergedContentFallback(&c); err != nil {
		return nil, err
	}
	db.populateClusterMeta(&c)
	return &c, nil
}

// UpdateClusterStatus updates the status of a cluster.
func (db *DB) UpdateClusterStatus(clusterID int64, status string) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE clusters SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), clusterID,
	)
	return err
}

// UpdateClusterMergedContent writes the AI-fused content to a cluster.
func (db *DB) UpdateClusterMergedContent(clusterID int64, title, summary, content string) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE clusters SET merged_title = ?, merged_summary = ?, merged_content = ?, updated_at = ? WHERE id = ?`,
		title, summary, content, time.Now(), clusterID,
	)
	return err
}

// UpdateClusterArticleCount updates the article_count for a cluster.
func (db *DB) UpdateClusterArticleCount(clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE clusters SET article_count = (SELECT COUNT(*) FROM articles WHERE cluster_id = ?), updated_at = ? WHERE id = ?`,
		clusterID, time.Now(), clusterID,
	)
	return err
}

// GetClustersByStatus retrieves clusters by status for a user.
func (db *DB) GetClustersByStatus(userID int64, status string) ([]models.Cluster, error) {
	db.WaitForReady()
	query := `
		SELECT id, user_id, status, merged_title, merged_summary,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, created_at, updated_at, is_read, is_favorite, is_read_later, is_hidden
FROM clusters WHERE user_id = ? AND status = ?
	`
	args := []any{userID, status}
	if status == "pending_merge" {
		query = `
		SELECT id, user_id, status, merged_title, merged_summary,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, created_at, updated_at, is_read, is_favorite, is_read_later, is_hidden
FROM clusters WHERE user_id = ? AND status IN ('pending_merge', 'merging')
	`
		args = []any{userID}
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []models.Cluster
	for rows.Next() {
		var c models.Cluster
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary,
			&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
			&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
		); err != nil {
			log.Printf("Error scanning cluster: %v", err)
			continue
		}
		clusters = append(clusters, c)
	}
	return clusters, nil
}

// GetArticlesByClusterID retrieves all articles belonging to a cluster.
func (db *DB) GetArticlesByClusterID(clusterID int64) ([]models.Article, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT a.id, a.feed_id, a.title, a.url, COALESCE(a.image_url, ''), a.published_at, a.summary, f.title, a.author, COALESCE(a.translated_title, '')
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		WHERE a.cluster_id = ?
		ORDER BY a.published_at DESC
	`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.Article
	for rows.Next() {
		var a models.Article
		var publishedAt sql.NullTime
		var summary, feedTitle, author sql.NullString
		if err := rows.Scan(&a.ID, &a.FeedID, &a.Title, &a.URL, &a.ImageURL, &publishedAt, &summary, &feedTitle, &author, &a.TranslatedTitle); err != nil {
			log.Printf("Error scanning cluster article: %v", err)
			continue
		}
		if publishedAt.Valid {
			a.PublishedAt = publishedAt.Time
		}
		a.Summary = summary.String
		a.FeedTitle = feedTitle.String
		a.Author = author.String
		a.ClusterID = clusterID
		articles = append(articles, a)
	}
	return articles, nil
}

// UpdateArticleClusterID assigns an article to a cluster.
func (db *DB) UpdateArticleClusterID(articleID, clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE articles SET cluster_id = ? WHERE id = ?`, clusterID, articleID)
	return err
}

// UpdateArticleSimHash stores SimHash data for an article.
func (db *DB) UpdateArticleSimHash(articleID int64, hash64 int64, b1, b2, b3, b4 int16) error {
	db.WaitForReady()
	_, err := db.Exec(
		`UPDATE articles SET simhash_64 = ?, simhash_b1 = ?, simhash_b2 = ?, simhash_b3 = ?, simhash_b4 = ? WHERE id = ?`,
		hash64, b1, b2, b3, b4, articleID,
	)
	return err
}

// FindSimHashCandidates finds articles with matching SimHash bands (pigeonhole principle).
func (db *DB) FindSimHashCandidates(userID int64, b1, b2, b3, b4 int16) ([]struct {
	ArticleID int64
	SimHash64 int64
	ClusterID int64
}, error) {
	db.WaitForReady()
	rows, err := db.Query(`
		SELECT id, simhash_64, cluster_id FROM articles
		WHERE user_id = ? AND cluster_id IS NOT NULL
		AND (simhash_b1 = ? OR simhash_b2 = ? OR simhash_b3 = ? OR simhash_b4 = ?)
	`, userID, b1, b2, b3, b4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []struct {
		ArticleID int64
		SimHash64 int64
		ClusterID int64
	}
	for rows.Next() {
		var c struct {
			ArticleID int64
			SimHash64 int64
			ClusterID int64
		}
		if err := rows.Scan(&c.ArticleID, &c.SimHash64, &c.ClusterID); err != nil {
			continue
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// FindSemanticCandidates uses sqlite-vec ANN search to find semantically similar articles.
func (db *DB) FindSemanticCandidates(userID int64, summaryEmbBlob []byte, topK int) ([]struct {
	ArticleID int64
	ClusterID int64
	Distance  float64
}, error) {
	db.WaitForReady()
	if topK <= 0 {
		topK = 10
	}
	rows, err := db.Query(`
		SELECT ae.article_id, a.cluster_id, ae.distance
		FROM article_embeddings ae
		JOIN articles a ON ae.article_id = a.id
		WHERE a.user_id = ? AND a.cluster_id IS NOT NULL
		AND ae.summary_embedding MATCH ? AND k = ?
		ORDER BY ae.distance
	`, userID, summaryEmbBlob, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		ArticleID int64
		ClusterID int64
		Distance  float64
	}
	for rows.Next() {
		var r struct {
			ArticleID int64
			ClusterID int64
			Distance  float64
		}
		if err := rows.Scan(&r.ArticleID, &r.ClusterID, &r.Distance); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// UpdateClusterEmbeddings stores embeddings for a cluster.
func (db *DB) UpdateClusterEmbeddings(clusterID int64, titleEmb, summaryEmb []byte) error {
	db.WaitForReady()
	titleEmb, summaryEmb = ensureVecColumnBlobs(titleEmb, summaryEmb)
	return db.WithWriteTx(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM cluster_embeddings WHERE cluster_id = ?`, clusterID); err != nil {
			return fmt.Errorf("delete cluster embedding: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO cluster_embeddings (cluster_id, title_embedding, summary_embedding) VALUES (?, ?, ?)`,
			clusterID, titleEmb, summaryEmb,
		); err != nil {
			return fmt.Errorf("insert cluster embedding: %w", err)
		}
		return nil
	})
}

// GetClustersForUser retrieves clusters for a user with filtering and pagination.
func (db *DB) GetClustersForUser(userID int64, filter string, feedID int64, category string, limit, offset int) ([]models.Cluster, error) {
	db.WaitForReady()

	baseQuery := `SELECT DISTINCT c.id, c.user_id, c.status, c.merged_title, c.merged_summary,
recommendation_archive_date, recommendation_score, is_ai_recommended, recommendation_profile_id,
article_count, c.created_at, c.updated_at, c.is_read, c.is_favorite, c.is_read_later, c.is_hidden
FROM clusters c`
	args := []interface{}{}
	conditions := []string{"c.user_id = ?", "c.is_hidden = 0"}
	args = append(args, userID)

	if feedID > 0 || category != "" {
		baseQuery += `
JOIN articles a ON a.cluster_id = c.id
JOIN feeds f ON f.id = a.feed_id`
		if feedID > 0 {
			conditions = append(conditions, "a.feed_id = ?")
			args = append(args, feedID)
		}
		if category != "" {
			conditions = append(conditions, `(COALESCE(f.category, '') = ? OR COALESCE(f.category, '') LIKE ?)`)
			args = append(args, category, category+"/%")
		}
	}

	switch filter {
	case "unread":
		conditions = append(conditions, "c.is_read = 0")
	case "favorites":
		conditions = append(conditions, "c.is_favorite = 1")
	case "readLater":
		conditions = append(conditions, "c.is_read_later = 1")
	}

	baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	baseQuery += " ORDER BY c.updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []models.Cluster
	for rows.Next() {
		var c models.Cluster
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Status, &c.MergedTitle, &c.MergedSummary,
			&c.RecommendationArchiveDate, &c.RecommendationScore, &c.IsAIRecommended, &c.RecommendationProfileID,
			&c.ArticleCount, &c.CreatedAt, &c.UpdatedAt, &c.IsRead, &c.IsFavorite, &c.IsReadLater, &c.IsHidden,
		); err != nil {
			log.Printf("Error scanning cluster: %v", err)
			continue
		}
		clusters = append(clusters, c)
	}
	db.populateClustersMeta(clusters)
	return clusters, nil
}

func chooseClusterDisplayTitle(mergedTitle, translatedTitle, articleTitle string) string {
	mergedTitle = strings.TrimSpace(mergedTitle)
	translatedTitle = strings.TrimSpace(translatedTitle)
	articleTitle = strings.TrimSpace(articleTitle)

	if translatedTitle != "" {
		return translatedTitle
	}
	if mergedTitle != "" {
		return mergedTitle
	}
	return articleTitle
}

func chooseClusterSingleArticleDisplayTitle(db *DB, userID int64, article models.Article, mergedTitle string) string {
	resolvedTitle := db.ResolveArticleTitleForCluster(userID, article)
	return chooseClusterDisplayTitle(mergedTitle, resolvedTitle, article.Title)
}

func (db *DB) applyClusterMergedContentFallback(c *models.Cluster) error {
	if c == nil || c.ID <= 0 || c.UserID <= 0 {
		return nil
	}
	if strings.TrimSpace(c.MergedTitle) != "" &&
		strings.TrimSpace(c.MergedSummary) != "" &&
		strings.TrimSpace(c.MergedContent) != "" {
		return nil
	}

	title, summary, content, ok, err := db.buildClusterFallbackFields(c.UserID, c.ID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if strings.TrimSpace(c.MergedTitle) == "" {
		c.MergedTitle = title
	}
	if strings.TrimSpace(c.MergedSummary) == "" {
		c.MergedSummary = summary
	}
	if strings.TrimSpace(c.MergedContent) == "" {
		c.MergedContent = content
	}
	return nil
}

type clusterMetaArticle struct {
	ArticleID         int64
	FeedID            int64
	FeedTitle         string
	Author            string
	ArticleTitle      string
	TranslatedTitle   string
	ImageURL          string
	TranslateArticles bool
}

type clusterMetaAccumulator struct {
	FeedSet      map[string]bool
	AuthorSet    map[string]bool
	ArticleCount int
	Latest       clusterMetaArticle
}

func (db *DB) populateClustersMeta(clusters []models.Cluster) {
	if len(clusters) == 0 {
		return
	}

	clusterRefs := make([]*models.Cluster, 0, len(clusters))
	for i := range clusters {
		clusterRefs = append(clusterRefs, &clusters[i])
	}
	db.populateClusterRefsMeta(clusterRefs)
}

func (db *DB) populateClusterScoreMeta(results []ClusterWithScore) {
	if len(results) == 0 {
		return
	}

	clusterRefs := make([]*models.Cluster, 0, len(results))
	for i := range results {
		clusterRefs = append(clusterRefs, &results[i].Cluster)
	}
	db.populateClusterRefsMeta(clusterRefs)
}

func (db *DB) populateClusterRefsMeta(clusters []*models.Cluster) {
	if len(clusters) == 0 {
		return
	}

	clusterByID := make(map[int64]*models.Cluster, len(clusters))
	clusterIDs := make([]int64, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster == nil || cluster.ID <= 0 {
			continue
		}
		cluster.FeedTitles = nil
		cluster.Authors = nil
		cluster.ImageURL = ""
		cluster.DisplayTitle = ""
		clusterByID[cluster.ID] = cluster
		clusterIDs = append(clusterIDs, cluster.ID)
	}
	if len(clusterIDs) == 0 {
		return
	}

	placeholders := make([]string, len(clusterIDs))
	args := make([]interface{}, len(clusterIDs))
	for i, clusterID := range clusterIDs {
		placeholders[i] = "?"
		args[i] = clusterID
	}

	rows, err := db.Query(fmt.Sprintf(`
		SELECT a.cluster_id, a.id, a.feed_id, COALESCE(f.title, ''), COALESCE(a.author, ''),
		       COALESCE(a.title, ''), COALESCE(a.translated_title, ''), COALESCE(a.image_url, ''),
		       COALESCE(f.translate_articles, 0)
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		WHERE a.cluster_id IN (%s)
		ORDER BY a.cluster_id, a.published_at DESC, a.id DESC
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return
	}
	defer rows.Close()

	metaByClusterID := make(map[int64]*clusterMetaAccumulator, len(clusterIDs))
	for rows.Next() {
		var clusterID int64
		var article clusterMetaArticle
		if err := rows.Scan(
			&clusterID,
			&article.ArticleID,
			&article.FeedID,
			&article.FeedTitle,
			&article.Author,
			&article.ArticleTitle,
			&article.TranslatedTitle,
			&article.ImageURL,
			&article.TranslateArticles,
		); err != nil {
			continue
		}

		cluster := clusterByID[clusterID]
		if cluster == nil {
			continue
		}
		meta := metaByClusterID[clusterID]
		if meta == nil {
			meta = &clusterMetaAccumulator{
				FeedSet:   make(map[string]bool),
				AuthorSet: make(map[string]bool),
			}
			metaByClusterID[clusterID] = meta
		}

		meta.ArticleCount++
		if meta.ArticleCount == 1 {
			meta.Latest = article
		}
		if cluster.ImageURL == "" && article.ImageURL != "" {
			cluster.ImageURL = article.ImageURL
		}
		if article.FeedTitle != "" && !meta.FeedSet[article.FeedTitle] {
			meta.FeedSet[article.FeedTitle] = true
			cluster.FeedTitles = append(cluster.FeedTitles, article.FeedTitle)
		}
		if article.Author != "" && !meta.AuthorSet[article.Author] {
			meta.AuthorSet[article.Author] = true
			cluster.Authors = append(cluster.Authors, article.Author)
		}
	}

	cachedSingleTitles := db.resolveCachedSingleClusterTitles(metaByClusterID, clusterByID)
	for clusterID, cluster := range clusterByID {
		meta := metaByClusterID[clusterID]
		if meta == nil || meta.ArticleCount <= 1 {
			latest := clusterMetaArticle{}
			if meta != nil {
				latest = meta.Latest
			}
			resolvedTitle := resolveBatchClusterArticleTitle(latest, cachedSingleTitles)
			cluster.DisplayTitle = chooseClusterDisplayTitle(cluster.MergedTitle, resolvedTitle, latest.ArticleTitle)
			continue
		}
		cluster.DisplayTitle = chooseClusterDisplayTitle(cluster.MergedTitle, "", meta.Latest.ArticleTitle)
	}
}

func (db *DB) resolveCachedSingleClusterTitles(metaByClusterID map[int64]*clusterMetaAccumulator, clusterByID map[int64]*models.Cluster) map[int64]string {
	if len(metaByClusterID) == 0 {
		return nil
	}

	var userID int64
	for _, cluster := range clusterByID {
		if cluster != nil && cluster.UserID > 0 {
			userID = cluster.UserID
			break
		}
	}
	if userID <= 0 {
		return nil
	}

	targetLang, _ := db.GetSettingWithFallback(userID, "target_language")
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = defaultClusterTitleTargetLanguage
	}
	provider, _ := db.GetSettingWithFallback(userID, "translation_provider")
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = defaultClusterTitleTranslationProvider
	}

	type pendingTitle struct {
		article clusterMetaArticle
		hash    string
	}
	pending := make([]pendingTitle, 0)
	hashes := make([]string, 0)
	for _, meta := range metaByClusterID {
		if meta == nil || meta.ArticleCount > 1 || !meta.Latest.TranslateArticles {
			continue
		}
		if strings.TrimSpace(meta.Latest.TranslatedTitle) != "" || strings.TrimSpace(meta.Latest.ArticleTitle) == "" {
			continue
		}
		hash := hashClusterTitleTranslation(meta.Latest.ArticleTitle)
		pending = append(pending, pendingTitle{article: meta.Latest, hash: hash})
		hashes = append(hashes, hash)
	}
	if len(pending) == 0 {
		return nil
	}

	translations, err := db.GetCachedTranslations(hashes, targetLang, provider)
	if err != nil || len(translations) == 0 {
		return nil
	}

	resolved := make(map[int64]string, len(translations))
	for _, item := range pending {
		cachedTitle := strings.TrimSpace(translations[item.hash])
		if cachedTitle == "" {
			continue
		}
		resolved[item.article.ArticleID] = cachedTitle
		if item.article.ArticleID > 0 {
			_ = db.UpdateArticleTranslation(item.article.ArticleID, cachedTitle)
		}
	}
	return resolved
}

func resolveBatchClusterArticleTitle(article clusterMetaArticle, cachedTitles map[int64]string) string {
	if title := strings.TrimSpace(article.TranslatedTitle); title != "" {
		return title
	}
	if cachedTitles != nil {
		if title := strings.TrimSpace(cachedTitles[article.ArticleID]); title != "" {
			return title
		}
	}
	return strings.TrimSpace(article.ArticleTitle)
}

// populateClusterMeta populates FeedTitles and Authors for a cluster.
func (db *DB) populateClusterMeta(c *models.Cluster) {
	rows, err := db.Query(`
		SELECT a.id, a.feed_id, COALESCE(f.title, ''), COALESCE(a.author, ''), COALESCE(a.title, ''), COALESCE(a.translated_title, ''), COALESCE(a.image_url, '')
		FROM articles a
		LEFT JOIN feeds f ON a.feed_id = f.id
		WHERE a.cluster_id = ?
		ORDER BY a.published_at DESC, a.id DESC
	`, c.ID)
	if err != nil {
		return
	}
	defer rows.Close()

	feedSet := make(map[string]bool)
	authorSet := make(map[string]bool)
	articleCount := 0
	var latestArticle models.Article
	c.ImageURL = ""
	for rows.Next() {
		var articleID, feedID int64
		var feedTitle, author, articleTitle, translatedTitle, imageURL string
		if err := rows.Scan(&articleID, &feedID, &feedTitle, &author, &articleTitle, &translatedTitle, &imageURL); err != nil {
			continue
		}
		articleCount++
		if articleCount == 1 {
			latestArticle = models.Article{
				ID:              articleID,
				FeedID:          feedID,
				UserID:          c.UserID,
				Title:           articleTitle,
				TranslatedTitle: translatedTitle,
			}
		}
		if c.ImageURL == "" && imageURL != "" {
			c.ImageURL = imageURL
		}
		if feedTitle != "" && !feedSet[feedTitle] {
			feedSet[feedTitle] = true
			c.FeedTitles = append(c.FeedTitles, feedTitle)
		}
		if author != "" && !authorSet[author] {
			authorSet[author] = true
			c.Authors = append(c.Authors, author)
		}
	}

	if articleCount <= 1 {
		c.DisplayTitle = chooseClusterSingleArticleDisplayTitle(db, c.UserID, latestArticle, c.MergedTitle)
		return
	}
	c.DisplayTitle = chooseClusterDisplayTitle(c.MergedTitle, "", latestArticle.Title)
}

// MarkClusterRead marks a cluster as read/unread.
func (db *DB) MarkClusterRead(clusterID int64, read bool) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE clusters SET is_read = ?, updated_at = ? WHERE id = ?`, read, time.Now(), clusterID)
	return err
}

// MarkAllClustersReadForUser marks all visible clusters as read for a user.
func (db *DB) MarkAllClustersReadForUser(userID int64, filter string, feedID int64, category string) error {
	db.WaitForReady()

	query := `UPDATE clusters SET is_read = 1, updated_at = ? WHERE id IN (
SELECT DISTINCT c.id
FROM clusters c`
	args := []interface{}{time.Now()}
	conditions := []string{"c.user_id = ?", "c.is_hidden = 0"}
	args = append(args, userID)

	if feedID > 0 || category != "" {
		query += `
JOIN articles a ON a.cluster_id = c.id
JOIN feeds f ON f.id = a.feed_id`
		if feedID > 0 {
			conditions = append(conditions, "a.feed_id = ?")
			args = append(args, feedID)
		}
		if category != "" {
			conditions = append(conditions, `(COALESCE(f.category, '') = ? OR COALESCE(f.category, '') LIKE ?)`)
			args = append(args, category, category+"/%")
		}
	}

	switch filter {
	case "unread":
		conditions = append(conditions, "c.is_read = 0")
	case "favorites":
		conditions = append(conditions, "c.is_favorite = 1")
	case "readLater":
		conditions = append(conditions, "c.is_read_later = 1")
	}

	query += " WHERE " + strings.Join(conditions, " AND ") + ")"

	_, err := db.Exec(query, args...)
	return err
}

// ToggleClusterFavorite toggles the favorite status of a cluster.
func (db *DB) ToggleClusterFavorite(clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE clusters SET is_favorite = 1 - is_favorite, updated_at = ? WHERE id = ?`, time.Now(), clusterID)
	return err
}

// SetClusterFavorite sets the favorite status of a cluster.
func (db *DB) SetClusterFavorite(clusterID int64, favorite bool) error {
	db.WaitForReady()
	_, err := db.Exec(`UPDATE clusters SET is_favorite = ?, updated_at = ? WHERE id = ?`, favorite, time.Now(), clusterID)
	return err
}

// ToggleClusterReadLater toggles the read-later status of a cluster.
func (db *DB) ToggleClusterReadLater(clusterID int64) error {
	db.WaitForReady()
	_, err := db.Exec(`
		UPDATE clusters
		SET is_read_later = 1 - is_read_later,
			is_read = CASE WHEN is_read_later = 0 THEN 0 ELSE is_read END,
			updated_at = ?
		WHERE id = ?
	`, time.Now(), clusterID)
	return err
}
