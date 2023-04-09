package milvus

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type QA struct {
	ID    int64
	Q     string
	A     string
	Score float32
}

func Search(films []float64, addr string) []QA {
	// setup context for client creation, use 8 seconds here
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	c, err := client.NewGrpcClient(ctx, addr)
	if err != nil {
		// handling error and exit, to make example simple here
		log.Fatal("failed to connect to milvus:", err.Error())
	}
	// in a main func, remember to close the client
	defer func(c client.Client) {
		_ = c.Close()
	}(c)

	// here is the collection name we use in this example
	collectionName := `q_a_demo`
	// load collection with async=false
	err = c.LoadCollection(ctx, collectionName, false)
	if err != nil {
		log.Fatal("failed to load collection:", err.Error())
	}
	log.Println("load collection completed")

	var searchFilm []float32
	for i, film := range films {
		if i > 1023 {
			break
		}
		searchFilm = append(searchFilm, float32(film))
	}
	vector := entity.FloatVector(searchFilm[:])
	// Use flat search param
	sp, err := entity.NewIndexIvfFlatSearchParam(5)
	if err != nil {
		log.Fatal("fail to create flat search param:", err.Error())
	}
	sr, err := c.Search(
		ctx, collectionName,
		[]string{},
		"",
		[]string{"ID", "Q", "A"},
		[]entity.Vector{vector},
		"Vector",
		entity.L2,
		4,
		sp,
	)
	if err != nil {
		log.Fatal("fail to search collection:", err.Error())
	}

	fmt.Println(sr)

	var qas []QA
	for _, result := range sr {

		var idColumn *entity.ColumnInt64
		var qColumn *entity.ColumnVarChar
		var aColumn *entity.ColumnVarChar
		for _, field := range result.Fields {
			if field.Name() == "ID" {
				c, ok := field.(*entity.ColumnInt64)
				if ok {
					idColumn = c
				}
			}
			if field.Name() == "Q" {
				q, ok := field.(*entity.ColumnVarChar)
				if ok {
					qColumn = q
				}
			}
			if field.Name() == "A" {
				a, ok := field.(*entity.ColumnVarChar)
				if ok {
					aColumn = a
				}
			}
		}
		if idColumn == nil {
			log.Fatal("result field not math")
		}
		for i := 0; i < result.ResultCount; i++ {
			id, err := idColumn.ValueByIdx(i)
			if err != nil {
				log.Fatal(err.Error())
			}
			q, err := qColumn.ValueByIdx(i)
			if err != nil {
				log.Fatal(err.Error())
			}
			a, err := aColumn.ValueByIdx(i)
			if err != nil {
				log.Fatal(err.Error())
			}
			qa := new(QA)
			qa.ID = id
			qa.Q = q
			qa.A = a
			qa.Score = result.Scores[i]
			qas = append(qas, *qa)
		}
	}
	// clean up
	defer func(c client.Client, ctx context.Context, collName string) {
		_ = c.ReleaseCollection(ctx, collName)
	}(c, ctx, collectionName)

	return qas
}
