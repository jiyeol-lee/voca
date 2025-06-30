package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jiyeol-lee/copilot"
	"github.com/jiyeol-lee/voca/pkg/news"
	"github.com/jiyeol-lee/voca/pkg/vocabulary"
)

var vocaGpt = struct {
	model         string
	systemContent string
}{
	model: "gpt-4.1",
	systemContent: `You must:
- Keep your answers impersonal.
- Use actual line breaks in your responses; only use "\n" when you want a literal backslash followed by 'n'.
- Use Markdown formatting in your answers.
- Do not ask any follow-up questions.
- Answer, "I cannot answer because it is not a word or phrase," if it is not related to the word or phrase.
- Only answer two categories: **Explanation** and **Examples** as H2 headers.
- Each of the two categories above has two subcategories: **English** and **Korean** as H3 headers.
- For the Explanation category, add another subcategory: **Pronunciation** as an H4 header, only under the **Korean** subcategory. This category is for how to read English words or phrases using Korean
- Do not include any additional text or explanations outside of the specified categories.
- Translate into pure Korean without using any English words.

When given a word or phrase, follow these steps:
1. **Read the Text**: Carefully read the provided text.  
2. **Explain the Word or Phrase**: Briefly explain the meaning of the word or phrase in English and Korean, using clear language.  
3. **Provide Examples**: Give five examples of how the word or phrase is used in sentences, ensuring they are relevant and illustrative in English and Korean.`,
}

func main() {
	flag.Parse()
	args := flag.Args()

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
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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

	case "study":
		s := vocabulary.NewStore()

		var content string
		if len(args) > 1 {
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

		c, err := copilot.NewCopilot()
		if err != nil {
			log.Fatalf("Error creating copilot instance: %v", err)
		}

		res, err := c.ChatCompletion(vocaGpt.model, []map[string]string{
			{"role": "system", "content": vocaGpt.systemContent},
			{"role": "user", "content": content},
		})
		if err != nil {
			log.Fatalf("Error getting chat completion: %v", err)
		}

		go func() {
			cmd := exec.Command("say", content)
			cmd.Output()
		}()
		err = pagerView(res.Choices[0].Message.Content)
		if err != nil {
			log.Fatalf("Error displaying content in pager view: %v", err)
		}
	default:
		fmt.Println("Expected 'news', 'add' or 'study' subcommands")
		os.Exit(1)
	}
}
