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
	model: "gpt-4.1-nano",
	systemContent: `You must:
- Keep your answers impersonal.
- Use actual line breaks in your responses; only use "\n" when you want a literal backslash followed by 'n'.
- Use Markdown formatting in your answers.
- Do not ask any follow-up questions.
- Answer, "I cannot answer because it is not a word or phrase," if it is not related to the word or phrase.
- Add a word or phrase as H1 header in your response.
- Only answer two categories: **Explanation** and **Examples** as H2 headers.
- Each of the two categories above has two subcategories: **English** and **Korean** as H3 headers.
- For the Explanation category, add another subcategory: **Pronunciation** as an H4 header, only under the **Korean** subcategory. This category is for how to read English words or phrases using Korean
- Do not include any additional text or explanations outside of the specified categories.
- Translate into pure Korean without using any English words.

When given a word or phrase, follow these steps:
1. **Read the Text**: Carefully read the provided text.  
2. **Explain the Word or Phrase**: Briefly explain the meaning of the word or phrase in English and Korean, using clear language.  
3. **Provide Examples**: Give five examples of how the word or phrase is used in sentences, ensuring they are relevant and illustrative in English and Korean.`,
	temperature: 1,
}

var vocaStoryGpt = VocaGpt{
	model: "gpt-4.1-nano",
	systemContent: `You must:
- Keep your answers impersonal.
- Use actual line breaks in your responses; only use "\n" when you want a literal backslash followed by 'n'.
- Use Markdown formatting in your answers.
- Do not ask any follow-up questions.
- With the given words, create a story to help remember the words.
- Wrap given words in double asterisks in the story like this: **WORD** in both "Story (English)" and "한국어 번역". Do not wrap it in "Selected Words" section.
- Do not include any additional text or explanations outside of the specified guidelines.

Follow these schemas in your response (full capital letters are placeholders to be replaced with actual content):

# [STORY_TITLE] (KOREAN_TRANSLATION)

## Selected Words

- [WORD_1] (KOREAN_TRANSLATION)
- [WORD_2] (KOREAN_TRANSLATION)
- ...

## Story (English)

[STORY_IN_ENGLISH]

## 한국어 번역

[STORY_IN_KOREAN]`,
	temperature: 1,
}

func main() {
	flag.Parse()
	args := flag.Args()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable not set")
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
		fmt.Println("")
		if err := client.CreateChatCompletionStreamWithMarkdown(context.Background(), req, os.Stdout, opts); err != nil {
			log.Fatalf("stream error: %v", err)
		}

	case "study":
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
		fmt.Println("")
		if err := client.CreateChatCompletionStreamWithMarkdown(context.Background(), req, os.Stdout, opts); err != nil {
			log.Fatalf("stream error: %v", err)
		}
	default:
		fmt.Println("Expected 'news', 'add', 'delete' or 'study' subcommands")
		os.Exit(1)
	}
}
