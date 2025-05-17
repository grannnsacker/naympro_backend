package esearch

import (
	"bytes"
	"context"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"io"
	"log"
	"strconv"
)

// IndexJobsAsDocuments index jobs as documents
func (client ESClient) IndexJobsAsDocuments(ctx context.Context) error {
	jobs := ctx.Value(JobKey).([]Job)

	bulkIndexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:      "jobs",
		Client:     client.client,
		NumWorkers: 5,
	})
	if err != nil {
		return err
	}

	for _, document := range jobs {
		body, err := readerToReadSeeker(esutil.NewJSONReader(document))
		if err != nil {
			return err
		}
		err = bulkIndexer.Add(
			ctx,
			esutil.BulkIndexerItem{
				Action:     "index",
				DocumentID: strconv.Itoa(int(document.ID)),
				Body:       body,
			},
		)
		if err != nil {
			return err
		}
	}

	bulkIndexer.Close(ctx)
	biStats := bulkIndexer.Stats()
	log.Printf("Jobs indexed on Elasticsearch: %d \n", biStats.NumIndexed)
	return nil
}

// IndexJobAsDocument index one job as document
func (client ESClient) IndexJobAsDocument(documentID int, job Job) error {
	_, err := client.client.Index("jobs", esutil.NewJSONReader(job),
		client.client.Index.WithDocumentID(strconv.Itoa(documentID)))
	if err != nil {
		return err
	}
	return nil
}

// UpdateJobDocument update one job document
func (client ESClient) UpdateJobDocument(documentID string, updatedJob Job) error {
	data := map[string]interface{}{
		"doc": updatedJob,
	}
	_, err := client.client.Update("jobs", documentID, esutil.NewJSONReader(data))
	if err != nil {
		return err
	}
	return nil
}

// DeleteJobDocument delete document from the index
func (client ESClient) DeleteJobDocument(documentID string) error {
	_, err := client.client.Delete("jobs", documentID)
	if err != nil {
		return err
	}
	return nil
}

func readerToReadSeeker(reader io.Reader) (io.ReadSeeker, error) {
	// Read the entire content of the reader into a buffer.
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Create a new io.ReadSeeker from the buffer.
	readSeeker := bytes.NewReader(data)
	return readSeeker, nil
}
