package vocabulary

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jiyeol-lee/csvstore"
)

var vocabularyTableName = "eng__voca"

type store struct {
	cs        *csvstore.CSVStore
	storePath string
}

func NewStore() *store {
	return &store{
		cs:        nil,
		storePath: "",
	}
}

func (s *store) AddVocabulary(word string) (csvstore.CSVRecord, error) {
	cs, err := s.getCSVStore()
	if err != nil {
		return nil, fmt.Errorf("error getting CSV store: %w", err)
	}

	lowercaseWord := strings.ToLower(strings.TrimSpace(word))

	qResult, err := cs.Query(vocabularyTableName, []csvstore.QueryCondition{{
		Column:   "word",
		Operator: "=",
		Value:    lowercaseWord,
	}})
	if err != nil {
		return nil, fmt.Errorf("error while checking existing vocabulary: %w", err)
	}
	if qResult.Count > 0 {
		return nil, fmt.Errorf("vocabulary already exists: %s", word)
	}

	newVocab, err := cs.Insert(vocabularyTableName, csvstore.CSVRecord{
		"word":       lowercaseWord,
		"read_count": "0",
	})
	if err != nil {
		return nil, fmt.Errorf("error inserting new vocabulary: %w", err)
	}

	defer func() {
		err := s.syncStore()
		if err != nil {
			log.Printf("error syncing store: %v\n", err)
		}
	}()

	return newVocab, nil
}

func (s *store) DeleteVocabulary(word string) error {
	cs, err := s.getCSVStore()
	if err != nil {
		return fmt.Errorf("error getting CSV store: %w", err)
	}

	lowercaseWord := strings.ToLower(strings.TrimSpace(word))

	qResult, err := cs.Query(vocabularyTableName, []csvstore.QueryCondition{{
		Column:   "word",
		Operator: "=",
		Value:    lowercaseWord,
	}})
	if err != nil {
		return fmt.Errorf("error while checking existing vocabulary: %w", err)
	}
	if qResult.Count == 0 {
		return fmt.Errorf("vocabulary not found: %s", word)
	}

	qResult, err = cs.Delete(vocabularyTableName, []csvstore.QueryCondition{
		{
			Column:   "word",
			Operator: "=",
			Value:    lowercaseWord,
		},
	})
	if err != nil {
		return fmt.Errorf("error deleting vocabulary: %w", err)
	}

	defer func() {
		err := s.syncStore()
		if err != nil {
			log.Printf("error syncing store: %v\n", err)
		}
	}()

	return nil
}

type selectedWord struct {
	id        string
	word      string
	readCount string
}

func (s *store) GetRandomWords(limit int) ([]string, error) {
	cs, err := s.getCSVStore()
	if err != nil {
		return nil, fmt.Errorf("error getting CSV store: %w", err)
	}

	qResults, err := cs.Query(vocabularyTableName, []csvstore.QueryCondition{
		{
			Column:   "word",
			Operator: "!=",
			Value:    "",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error getting vocabulary: %w", err)
	}
	resultsLen := qResults.Count
	if resultsLen == 0 {
		return []string{}, nil
	}
	if limit > resultsLen {
		limit = resultsLen
	}
	perm := rand.Perm(resultsLen)
	selectedWords := make([]selectedWord, 0, limit)
	for _, idx := range perm[:limit] {
		record := qResults.Records[idx]
		selectedWords = append(selectedWords, selectedWord{
			id:        record["id"],
			word:      record["word"],
			readCount: record["read_count"],
		})
	}
	defer func() {
		for _, w := range selectedWords {
			oldReadCount, err := strconv.Atoi(w.readCount)
			if err != nil {
				log.Printf("error converting read_count to int: %v\n", err)
				continue
			}
			newReadCount := strconv.Itoa(oldReadCount + 1)
			cs.Update(vocabularyTableName, csvstore.CSVRecord{
				"read_count": newReadCount,
			}, []csvstore.QueryCondition{
				{
					Column:   "id",
					Operator: "=",
					Value:    w.id,
				},
			})
		}
		err = s.syncStore()
		if err != nil {
			log.Printf("error syncing store: %v\n", err)
		}
	}()

	words := make([]string, 0, limit)
	for _, w := range selectedWords {
		words = append(words, w.word)
	}
	return words, nil
}

func (s *store) GetLeastReadVocabulary() (csvstore.CSVRecord, error) {
	cs, err := s.getCSVStore()
	if err != nil {
		return nil, fmt.Errorf("error getting CSV store: %w", err)
	}

	qSortedResults, err := cs.QuerySortedRange(vocabularyTableName, "read_count", "asc", 1)
	if err != nil {
		return nil, fmt.Errorf("error getting vocabulary: %w", err)
	}
	if qSortedResults.Count == 0 {
		return nil, fmt.Errorf("no vocabulary found")
	}

	// why query again?
	// to pick one vocabulary within the least read count randomly
	leastReadCount := qSortedResults.Records[0]["read_count"]
	qResult, err := cs.Query(vocabularyTableName, []csvstore.QueryCondition{
		{
			Column:   "read_count",
			Operator: "=",
			Value:    leastReadCount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error querying vocabulary with least read count: %w", err)
	}
	if qResult.Count == 0 {
		return nil, fmt.Errorf("no vocabulary found with least read count: %s", leastReadCount)
	}

	// pick one integer from 0 to qResultsLen-1
	randomInt := rand.Intn(qResult.Count)
	leastReadVoca := qResult.Records[randomInt]
	defer func() {
		currentReadCountString, ok := leastReadVoca["read_count"]
		if !ok {
			log.Printf("read_count not found in leastReadVoca: %v\n", leastReadVoca)
		}
		currentReadCountInt, err := strconv.Atoi(currentReadCountString)
		if err != nil {
			log.Printf("error converting read_count to int: %v\n", err)
		}
		leastReadVoca["read_count"] = strconv.Itoa(currentReadCountInt + 1)
		cs.Update(vocabularyTableName, leastReadVoca, []csvstore.QueryCondition{
			{
				Column:   "id",
				Operator: "=",
				Value:    leastReadVoca["id"],
			},
		})

		err = s.syncStore()
		if err != nil {
			log.Printf("error syncing store: %v\n", err)
		}
	}()

	return leastReadVoca, nil
}

func (s *store) getCSVStore() (*csvstore.CSVStore, error) {
	if s.cs == nil {
		err := s.initialize()
		if err != nil {
			return nil, err
		}
	}
	return s.cs, nil
}

func (s *store) initialize() error {
	tempDir := os.TempDir()
	csvStoreFolderName := fmt.Sprintf("csv__voca--%s", time.Now().Format("2006-01-02"))
	csvStoreFilepath := filepath.Join(tempDir, csvStoreFolderName)
	if !checkIsFolderExists(csvStoreFilepath) {
		cmd := exec.Command(
			"git",
			"clone",
			"git@github.com:jiyeol-lee/csv__voca.git",
			csvStoreFolderName,
		)
		cmd.Dir = tempDir
		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("error cloning CSV store repository: %w", err)
		}
	}
	cs, err := csvstore.NewCSVStore(
		csvStoreFilepath,
	)
	if err != nil {
		return fmt.Errorf("error creating CSV store: %w", err)
	}

	if !cs.CheckTableExists(vocabularyTableName) {
		err = cs.CreateTable(
			vocabularyTableName,
			[]string{"id", "word", "read_count", "created_at", "updated_at"},
		)
		if err != nil {
			return fmt.Errorf("error creating vocabulary table: %w", err)
		}
	}

	s.cs = cs
	s.storePath = csvStoreFilepath
	return nil
}
