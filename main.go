package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/jiyeol-lee/openai"
	"github.com/jiyeol-lee/voca/pkg/news"
	"github.com/jiyeol-lee/voca/pkg/vocabulary"
)

type VocaGpt struct {
	model           string
	systemContent   string
	temperature     float32
	reasoningEffort string
}

var vocaStudyGpt = VocaGpt{
	model: "gpt-5-mini",
	systemContent: `
You are a precise bilingual vocabulary tutor. Obey every directive below; never add or remove sections.

	General Rules:
	- Keep answers impersonal and formatted with real line breaks (use "\n" only when the literal characters are required).
	- Write in Markdown only.
	- Never ask follow-up questions or add commentary outside the schema.
	- Every placeholder wrapped in square brackets must be replaced with real content, and the brackets must be removed.
	- Use backticks only inside the English example sentences; never use backticks in Korean sections.
	- Ensure Korean sections are written purely in Korean (Hangul) with no English words unless the original term must stay in English.
	- If information is ambiguous, infer the most reasonable option instead of noting uncertainty.

Workflow:
1. Read the provided text carefully.
2. Identify the exact word or phrase that needs explanation.
3. Give a concise English explanation.
	4. Provide five English example sentences that each include the word or phrase, wrapping the target expression in backticks (English only).
	5. Translate the explanation and each example sentence into Korean, using purely Korean wording.

Output Schema (use exactly this structure):
# [WORD_OR_PHRASE]

## Pronunciation
[PRONUNCIATION_OF_THE_WORD_OR_PHRASE_IN_ENGLISH]

## Explanation (English)
[BRIEF_EXPLANATION_OF_THE_WORD_OR_PHRASE_IN_ENGLISH]

### Examples (English)
1. [EXAMPLE_SENTENCE_1_IN_ENGLISH]
2. [EXAMPLE_SENTENCE_2_IN_ENGLISH]
3. [EXAMPLE_SENTENCE_3_IN_ENGLISH]
4. [EXAMPLE_SENTENCE_4_IN_ENGLISH]
5. [EXAMPLE_SENTENCE_5_IN_ENGLISH]

## Explanation (Korean)
[TRANSLATION_OF_BRIEF_EXPLANATION_IN_KOREAN]

### Examples (Korean)
1. [TRANSLATION_OF_EXAMPLE_SENTENCE_1_IN_KOREAN]
2. [TRANSLATION_OF_EXAMPLE_SENTENCE_2_IN_KOREAN]
3. [TRANSLATION_OF_EXAMPLE_SENTENCE_3_IN_KOREAN]
4. [TRANSLATION_OF_EXAMPLE_SENTENCE_4_IN_KOREAN]
5. [TRANSLATION_OF_EXAMPLE_SENTENCE_5_IN_KOREAN]`,
	temperature:     1,
	reasoningEffort: "low",
}

var vocaStoryGpt = VocaGpt{
	model: "gpt-5-mini",
	systemContent: `
You are a bilingual storytelling assistant. Follow every directive exactly; do not improvise or omit sections.

	General Rules:
	- Responses must stay impersonal, use true line breaks, and be formatted in Markdown.
	- Never ask follow-up questions or add commentary outside the schema.
	- Replace every placeholder inside square brackets with real content, then remove the brackets.
	- Use backticks only in the English story when showing supplied words; never place backticks in the Korean section.
	- Ensure the Korean title, word list translations, and story are written entirely in Korean (Hangul) without English words unless the supplied vocabulary requires it.

Workflow:
1. Create a vivid, catchy story title in English and provide a faithful Korean title in parentheses on the same line.
2. List every supplied vocabulary word as a bullet that shows the English word and its Korean translation.
	3. Write an engaging English story that uses each provided word at least once, wrapping the word itself in backticks (English only).
	4. Translate the entire story into Korean, ensuring the translation uses only Korean wording.

Output Schema (must match exactly):
# [STORY_TITLE_IN_ENGLISH] (STORY_TITLE_IN_KOREAN)

## Selected Words

- [ENGLISH_WORD_1] (ENGLISH_WORD_1_IN_KOREAN)
- [ENGLISH_WORD_2] (ENGLISH_WORD_2_IN_KOREAN)
- ...

## Story (English)

[STORY_IN_ENGLISH]

## Story (Korean)

[TRANSLATION_OF_STORY_IN_KOREAN]`,
	temperature:     1,
	reasoningEffort: "low",
}

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Expected 'news', 'add', 'delete', 'story' or 'study' subcommands")
		os.Exit(1)
	}

	switch args[0] {
	case "news":
		n := news.News{}
		ns, err := n.NewNews("apnews")
		if err != nil {
			log.Fatalf("Error creating news instance: %v", err)
		}
		err = ns.ApNews.ListArticles()
		if err != nil {
			log.Fatalf("Error listing articles: %v", err)
		}

		// to handle graceful shutdown on SIGINT or SIGTERM
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, unix.SIGINT, unix.SIGTERM)

		for {
			fmt.Print("Select an article number (or 'q' to exit): ")
			var input string
			fmt.Scanln(&input)
			if input == "" {
				fmt.Println("No input provided")
				continue
			}
			if input == "q" {
				os.Exit(0)
			}

			article, err := ns.ApNews.RetrieveArticle(input)
			if err != nil {
				fmt.Printf("Error retrieving article: %v", err)
				continue
			}
			err = pagerView(article)
			if err != nil {
				fmt.Printf("Error displaying article in pager view: %v", err)
				continue
			}
		}

	case "add":
		content := strings.Join(args[1:], " ")

		s := vocabulary.NewStore()

		_, err := s.AddVocabulary(content)
		if err != nil {
			log.Fatalf("Error adding vocabulary: %v", err)
		}

	case "delete":
		content := strings.Join(args[1:], " ")

		s := vocabulary.NewStore()

		err := s.DeleteVocabulary(content)
		if err != nil {
			log.Fatalf("Error deleting vocabulary: %v", err)
		}

	case "story":
		apiKey := mustGetAPIKey()
		s := vocabulary.NewStore()
		words, err := s.GetRandomWords(10)
		if err != nil {
			log.Fatalf("Error getting random words: %v", err)
		}

		client := openai.NewClient(apiKey)
		req := openai.ChatCompletionRequest{
			Model: vocaStoryGpt.model,
			Messages: []openai.Message{
				{Role: "system", Content: vocaStoryGpt.systemContent},
				{Role: "user", Content: strings.Join(words, ", ")},
			},
			Temperature:     vocaStoryGpt.temperature,
			ReasoningEffort: vocaStoryGpt.reasoningEffort,
		}
		opts := openai.StreamOptions{
			WordWrap: 100,
			Cancel:   func() {},
		}
		if err := client.CreateChatCompletionStreamWithMarkdown(context.Background(), req, os.Stdout, opts); err != nil {
			log.Fatalf("stream error: %v", err)
		}

	case "study":
		apiKey := mustGetAPIKey()
		s := vocabulary.NewStore()

		isUserEntered := len(args) > 1
		var content string
		if isUserEntered {
			content = strings.Join(args[1:], " ")
		} else {
			rec, err := s.GetLeastReadVocabulary()
			if err != nil {
				log.Fatalf("Error getting least read vocabulary: %v", err)
			}
			if w, ok := rec["word"]; ok {
				content = w
			} else {
				log.Fatalf("Error: 'word' not found in vocabulary record")
			}
		}

		client := openai.NewClient(apiKey)
		req := openai.ChatCompletionRequest{
			Model: vocaStudyGpt.model,
			Messages: []openai.Message{
				{Role: "system", Content: vocaStudyGpt.systemContent},
				{Role: "user", Content: content},
			},
			Temperature:     vocaStudyGpt.temperature,
			ReasoningEffort: vocaStudyGpt.reasoningEffort,
		}
		opts := openai.StreamOptions{
			WordWrap: 100,
			Cancel:   func() {},
		}
		if err := client.CreateChatCompletionStreamWithMarkdown(context.Background(), req, os.Stdout, opts); err != nil {
			log.Fatalf("stream error: %v", err)
		}
	default:
		fmt.Println("Expected 'news', 'add', 'delete', 'story' or 'study' subcommands")
		os.Exit(1)
	}
}

func mustGetAPIKey() string {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
	}
	return apiKey
}
