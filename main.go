package main

import (
    "bytes"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/gorilla/mux"
    "golang.org/x/net/html"
)

// Logger middleware to log requests
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Log the request details
        log.Printf("Started %s %s", r.Method, r.URL.Path)

        // Call the next handler
        next.ServeHTTP(w, r)

        // Log the response details
        duration := time.Since(start)
        log.Printf("Completed %s %s in %v", r.Method, r.URL.Path, duration)
    })
}

// formatJSON takes raw JSON bytes and returns formatted JSON bytes
func formatJSON(data []byte) ([]byte, error) {
    var parsedJSON interface{}
    if err := json.Unmarshal(data, &parsedJSON); err != nil {
        log.Printf("Error parsing JSON: %v", err)
        return nil, fmt.Errorf("failed to parse JSON: %w", err)
    }
    formattedJSON, err := json.MarshalIndent(parsedJSON, "", "  ")
    if err != nil {
        log.Printf("Error formatting JSON: %v", err)
        return nil, fmt.Errorf("failed to format JSON: %w", err)
    }
    return formattedJSON, nil
}

// formatXML takes raw XML bytes and returns formatted XML bytes
func formatXML(data []byte) ([]byte, error) {
    var parsedXML interface{}
    if err := xml.Unmarshal(data, &parsedXML); err != nil {
        log.Printf("Error parsing XML: %v", err)
        return nil, fmt.Errorf("failed to parse XML: %w", err)
    }
    formattedXML, err := xml.MarshalIndent(parsedXML, "", "  ")
    if err != nil {
        log.Printf("Error formatting XML: %v", err)
        return nil, fmt.Errorf("failed to format XML: %w", err)
    }
    return append(formattedXML, '\n'), nil
}

// formatHTML takes raw HTML bytes and returns formatted HTML bytes
func formatHTML(data []byte) ([]byte, error) {
    doc, err := html.Parse(bytes.NewReader(data))
    if err != nil {
        log.Printf("Error parsing HTML: %v", err)
        return nil, fmt.Errorf("failed to parse HTML: %w", err)
    }
    var buf bytes.Buffer
    if err := html.Render(&buf, doc); err != nil {
        log.Printf("Error rendering HTML: %v", err)
        return nil, fmt.Errorf("failed to render HTML: %w", err)
    }
    return buf.Bytes(), nil
}

// formatHandler handles the formatting logic based on the content type
func formatHandler(w http.ResponseWriter, r *http.Request) {
    // Extract the content type from query parameters
    contentType := strings.ToLower(r.URL.Query().Get("type"))
    if contentType == "" {
        log.Println("Missing 'type' parameter in request")
        http.Error(w, "Missing 'type' parameter. Specify 'json', 'xml', or 'html'.", http.StatusBadRequest)
        return
    }

    // Read the raw input data from the request body
    body, err := ioutil.ReadAll(r.Body)
    defer r.Body.Close()
    if err != nil {
        log.Printf("Error reading request body: %v", err)
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        return
    }

    // Log the received request data
    log.Printf("Received request with type=%s and body=%s", contentType, string(body))

    // Format the content based on the specified type
    var formattedData []byte
    switch contentType {
    case "json":
        formattedData, err = formatJSON(body)
    case "xml":
        formattedData, err = formatXML(body)
    case "html":
        formattedData, err = formatHTML(body)
    default:
        log.Printf("Invalid 'type' parameter: %s", contentType)
        http.Error(w, "Invalid 'type' parameter. Supported types are 'json', 'xml', and 'html'.", http.StatusBadRequest)
        return
    }

    if err != nil {
        log.Printf("Formatting failed: %v", err)
        http.Error(w, fmt.Sprintf("Formatting failed: %s", err.Error()), http.StatusInternalServerError)
        return
    }

    // Set the response headers and write the formatted data
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(formattedData)

    // Log the successful response
    log.Printf("Formatted data successfully returned for type=%s", contentType)
}

func main() {
    // Create a new router using Gorilla Mux
    r := mux.NewRouter()

    // Add logging middleware
    r.Use(Logger)

    // Define the endpoint for formatting
    r.HandleFunc("/format", formatHandler).Methods("POST")

    // Start the HTTP server
    log.Println("Server is running on http://localhost:8030")
    if err := http.ListenAndServe(":8030", r); err != nil {
        log.Fatalf("Server failed to start: %s", err)
    }
}