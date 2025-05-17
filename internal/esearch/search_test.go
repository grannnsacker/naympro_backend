package esearch

import (
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSearchJobs(t *testing.T) {
	c, err := elasticsearch.NewDefaultClient()
	require.NoError(t, err)
	client := ESClient{client: c}

	//jobID := 1
	//jobs := []Job{
	//	{
	//		ID:          int32(jobID),
	//		Title:       "Software Engineer",
	//		Description: "Job description...",
	//		Location:    "New York",
	//	},
	//{
	//	ID:          2,
	//	Title:       "Data Scientist",
	//	Description: "Data Scientist description...",
	//	Location:    "San Francisco",
	//},
	//}
	//err = client.IndexJobAsDocument(jobID, jobs[0])
	//require.NoError(t, err)
	//err = client.IndexJobAsDocument(2, jobs[1])
	//require.NoError(t, err)

	//ctx := context.Background()
	//results, err := client.SearchJobs(ctx, jobs[0].Title, 1, 10)
	//require.NoError(t, err)

	//require.Equal(t, jobs[0].ID, results[0].ID)
	//require.Equal(t, jobs[0].Title, results[0].Title)
	//require.Equal(t, jobs[0].Description, results[0].Description)
	//require.Equal(t, jobs[0].Location, results[0].Location)

	// GetDocumentIDByJobID tests
	//_, err = client.GetDocumentIDByJobID(jobID)
	//require.Error(t, err)

	documentID2, err := client.GetDocumentIDByJobID(9999999999)
	require.Error(t, err)
	require.Empty(t, documentID2)
}

func TestNewClient(t *testing.T) {
	c, err := elasticsearch.NewDefaultClient()
	assert.NoError(t, err)

	// Call the function being tested
	client := NewClient(c)

	require.Equal(t, c, client.(*ESClient).client, "ESearchClient should contain the same Elasticsearch client")
}
