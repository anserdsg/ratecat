package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

func main() {
	// 1. Create a cookie jar to manage cookies
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	// 2. Create an HTTP client with the cookie jar
	client := &http.Client{
		Jar:     jar,
		Timeout: time.Second * 30, // Set a timeout (optional but recommended)
	}

	// 3. Set up the request (mimicking a Chrome browser)
	req, err := http.NewRequest("GET", "https://www.google.com", nil) // Replace with your target URL
	if err != nil {
		panic(err)
	}

	// 4. Set common Chrome headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br") // If you want to handle compressed responses
	// Add other headers if needed...

	// 5. Add cookies (if you have any you want to send initially)
	// Example:
	// cookies := []*http.Cookie{
	//     {Name: "cookie_name", Value: "cookie_value", Domain: "www.example.com", Path: "/", Expires: time.Now().Add(24 * time.Hour)},
	// }
	// jar.SetCookies(req.URL, cookies)

	// 6. Perform the request
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// 7. Handle the response
	fmt.Println("Status Code:", resp.StatusCode)

	// 8. Print cookies received from the server
	fmt.Println("Cookies:")
	for _, cookie := range jar.Cookies(req.URL) {
		fmt.Printf("  %s: %s\n", cookie.Name, cookie.Value)
	}

	// 9. Read and process the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println("Response Body:", string(body))

	// 10. Example of making a second request (cookies will be handled automatically)
	// req2, _ := http.NewRequest("GET", "https://www.example.com/some/other/page", nil) // Another URL on the same domain
	// req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36")
	// resp2, _ := client.Do(req2)
	// defer resp2.Body.Close()
	// fmt.Println("Status Code (Second Request):", resp2.StatusCode)
}
