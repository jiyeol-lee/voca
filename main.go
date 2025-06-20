package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/jiyeol-lee/voca/pkg/copilot"
	"github.com/jiyeol-lee/voca/pkg/vocabulary"
)

func main() {
	flag.Parse()
	args := flag.Args()

	switch args[0] {
	case "add":
		content := strings.Join(args[1:], " ")

		voca := &vocabulary.Store{}

		_, err := voca.NewVocabulary(content)
		if err != nil {
			log.Fatalf("Error adding vocabulary: %v", err)
		}
		break
	case "study":
		voca := &vocabulary.Store{}

		rec, err := voca.GetLeastReadVocabulary()
		if err != nil {
			log.Fatalf("Error getting least read vocabulary: %v", err)
			return
		}
		content, ok := rec["word"]
		if !ok {
			log.Fatalf("Error: 'word' not found in vocabulary record")
		}

		c := copilot.Copilot{}

		res, err := c.ChatCompletion("gpt-4.1", []map[string]string{
			{
				"role": "system",
				"content": `You must:
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
			},
			{
				"role":    "user",
				"content": content,
			},
		})
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		var unmarshaledRes struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		err = json.Unmarshal([]byte(res), &unmarshaledRes)
		if err != nil {
			fmt.Println("Error unmarshaling response:", err)
			return
		}
		fmt.Println(unmarshaledRes.Choices[0].Message.Content)

		cmd := exec.Command("say", content)
		cmd.Output()
		break
	default:
		fmt.Println("Expected 'add' or 'study' subcommands")
		os.Exit(1)

	}
}
