package control

import (
	"context"
	"encoding/base64"

	"github.com/jackc/pgx/v5"
)

func (s *Store) hydrateRunInputImages(ctx context.Context, runs []Run) error {
	if len(runs) == 0 {
		return nil
	}
	runIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		runIDs = append(runIDs, run.ID)
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, run_id, filename, mime_type, size_bytes, created_at
		FROM run_input_images
		WHERE run_id = ANY($1)
		ORDER BY created_at ASC, id ASC`, runIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	images, err := scanRunInputImages(rows)
	if err != nil {
		return err
	}
	byRun := map[string][]RunInputImage{}
	for _, image := range images {
		byRun[image.RunID] = append(byRun[image.RunID], image)
	}
	for index := range runs {
		runs[index].InputImages = byRun[runs[index].ID]
	}
	return nil
}

func insertRunInputImagesTx(ctx context.Context, tx pgx.Tx, runID string, images []normalizedRunInputImage) ([]RunInputImage, error) {
	if len(images) == 0 {
		return nil, nil
	}
	out := make([]RunInputImage, 0, len(images))
	for _, image := range images {
		item, err := scanRunInputImage(tx.QueryRow(ctx, `
			INSERT INTO run_input_images (run_id, filename, mime_type, size_bytes, content)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, run_id, filename, mime_type, size_bytes, created_at`,
			runID, image.Filename, image.MimeType, len(image.Content), image.Content))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func loadRunInputImageAttachmentsTx(ctx context.Context, tx pgx.Tx, runID string) ([]RunInputImageAttachment, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, filename, mime_type, content
		FROM run_input_images
		WHERE run_id=$1
		ORDER BY created_at ASC, id ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RunInputImageAttachment
	for rows.Next() {
		var item RunInputImageAttachment
		var content []byte
		if err := rows.Scan(&item.ID, &item.Filename, &item.MimeType, &content); err != nil {
			return nil, err
		}
		item.ContentBase64 = base64.StdEncoding.EncodeToString(content)
		out = append(out, item)
	}
	return out, rows.Err()
}
