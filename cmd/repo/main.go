package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"time"
)

type Action struct {
	A string `json:"a"`
	N string `json:"n"`
	D string `json:"d"`
	I int    `json:"i"`
}

type Response struct {
	Data string `json:"data,omitempty"`
	Err  string `json:"err,omitempty"`
}
type repoClient struct {
	target string
	client *http.Client
}

func newRepoClient(target string) *repoClient {
	return &repoClient{
		target: target,
		client: &http.Client{},
	}
}

func (r *repoClient) post(action Action) (*Response, error) {
	data, err := json.Marshal(action)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, r.target, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("User-Agent", `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.182 Safari/537.36`)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to post: %v", err)
	}

	defer resp.Body.Close()

	// read response
	var respData Response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// fmt.Printf("status: %s\n", resp.Status)
	// fmt.Printf("body: %s\n", body)

	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &respData, nil
}

func (r *repoClient) add(name string, inputFile string, chunkSize int, pause time.Duration) error {
	_, err := r.post(Action{A: "d", N: name})
	if err != nil {
		return fmt.Errorf("failed to delete: %v", err)
	}

	// read file content
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// encode file content to base64 []byte
	encoded, err := encodeData(content)
	if err != nil {
		return fmt.Errorf("failed to encode data: %v", err)
	}

	chunks := make([][]byte, 0)
	for chunk := range slices.Chunk(encoded, chunkSize) {
		chunks = append(chunks, chunk)
	}

	totalChunks := len(chunks)

	fmt.Printf("Process '%s' file to '%s' object, chunkSize: %d, pause: %s\n", inputFile, name, chunkSize, pause)
	for i, chunk := range chunks {
		fmt.Printf("Processing part [%02d/%d]\n", i+1, totalChunks)
		_, err := r.post(Action{A: "a", N: name, D: string(chunk), I: i})
		if err != nil {
			return fmt.Errorf("failed to append: %v", err)
		}
		time.Sleep(pause)
	}

	fmt.Printf("Upload %s to %s completed\n", inputFile, name)

	return nil
}

func (r *repoClient) read(name string, outputFile string) error {
	resp, err := r.post(Action{A: "r", N: name})
	if err != nil {
		return fmt.Errorf("failed to read: %v", err)
	}

	if resp.Err != "" {
		return fmt.Errorf("failed to read: %s", resp.Err)
	}

	// decode base64 []byte to file content
	// decoded, err := decodeData([]byte(resp.Data))
	// if err != nil {
	// 	return fmt.Errorf("failed to decode data: %v", err)
	// }
	decoded := []byte(resp.Data)
	fmt.Printf("Writing %s object to %s\n", name, outputFile)
	if err := os.WriteFile(outputFile, decoded, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func decodeData(data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	if _, err := buf.ReadFrom(decoder); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func encodeData(data []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	buf.Grow(base64.StdEncoding.EncodedLen(len(data)))

	encoder := base64.NewEncoder(base64.StdEncoding, buf)
	_, err := encoder.Write(data)
	if err != nil {
		return nil, err
	}

	err = encoder.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func main() {
	var target string
	var action string
	var chunkSize int
	var pause time.Duration
	flag.StringVar(&target, "t", "", "endpoint target")
	flag.StringVar(&action, "a", "", "action, add/read/delete")
	flag.IntVar(&chunkSize, "c", 1024*64, "chunk size")
	flag.DurationVar(&pause, "p", 1000*time.Millisecond, "pause duration")

	flag.Usage = func() {
		fmt.Println("repo -t <target> -a <action> [args]")
		flag.PrintDefaults()
		fmt.Print(`actions:
  add    <name> <input_file>
  read   <name> <output_file>
  delete <name>
`)

		os.Exit(1)
	}

	flag.Parse()

	if target == "" {
		target = os.Getenv("TARGET")
	}

	if target == "" || action == "" {
		flag.Usage()
		return
	}

	r := newRepoClient(target)

	args := flag.Args()

	switch action {
	case "add":
		if len(args) != 2 {
			flag.Usage()
		}
		name := args[0]
		inputFile := args[1]

		if err := r.add(name, inputFile, chunkSize, pause); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "read":
		if len(args) != 2 {
			flag.Usage()
		}
		name := args[0]
		outputFile := args[1]

		if err := r.read(name, outputFile); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case "delete":
	default:
		flag.Usage()
	}
}
