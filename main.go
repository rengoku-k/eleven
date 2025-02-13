package main

import (
    "bytes"
    "encoding/json"
    "encoding/xml"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/gorilla/mux"
    "golang.org/x/net/html"
)

// Metrics struct to track /format API usage
type FormatAPIMetrics struct {
    RequestCount    int64         // Total number of requests
    ErrorCount      int64         // Total number of errors
    TotalDuration   time.Duration // Total time spent processing requests
    MaxPayloadSize  int64         // Largest payload size received
    mu              sync.Mutex    // Mutex for thread-safe updates
}

var formatMetrics = &FormatAPIMetrics{}

// Logger middleware to log requests
func Logger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Log the start of the request
        log.Printf("Started %s %s", r.Method, r.URL.Path)

        // Call the next handler
        next.ServeHTTP(w, r)

        // Log the completion of the request
        duration := time.Since(start)
        log.Printf("Completed %s %s in %v", r.Method, r.URL.Path, duration)
    })
}

// Middleware to track metrics for /format API
func FormatMetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        // Read the request body to calculate payload size
        body, err := ioutil.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "Failed to read request body", http.StatusBadRequest)
            return
        }
        r.Body = ioutil.NopCloser(bytes.NewBuffer(body)) // Restore the body for the next handler

        payloadSize := int64(len(body))

        // Call the next handler
        next.ServeHTTP(w, r)

        duration := time.Since(start)

        // Update metrics
        formatMetrics.mu.Lock()
        defer formatMetrics.mu.Unlock()
        formatMetrics.RequestCount++
        formatMetrics.TotalDuration += duration
        if payloadSize > formatMetrics.MaxPayloadSize {
            formatMetrics.MaxPayloadSize = payloadSize
        }
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
        formatMetrics.mu.Lock()
        formatMetrics.ErrorCount++
        formatMetrics.mu.Unlock()
        return
    }

    // Read the raw input data from the request body
    body, err := ioutil.ReadAll(r.Body)
    defer r.Body.Close()
    if err != nil {
        log.Printf("Error reading request body: %v", err)
        http.Error(w, "Failed to read request body", http.StatusBadRequest)
        formatMetrics.mu.Lock()
        formatMetrics.ErrorCount++
        formatMetrics.mu.Unlock()
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
        formatMetrics.mu.Lock()
        formatMetrics.ErrorCount++
        formatMetrics.mu.Unlock()
        return
    }

    if err != nil {
        log.Printf("Formatting failed: %v", err)
        http.Error(w, fmt.Sprintf("Formatting failed: %s", err.Error()), http.StatusInternalServerError)
        formatMetrics.mu.Lock()
        formatMetrics.ErrorCount++
        formatMetrics.mu.Unlock()
        return
    }

    // Set the response headers and write the formatted data
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(formattedData)

    // Log the successful response
    log.Printf("Formatted data successfully returned for type=%s", contentType)
}

// MetricsHandler exposes metrics for the /format API
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
    formatMetrics.mu.Lock()
    defer formatMetrics.mu.Unlock()

    averageDuration := time.Duration(0)
    if formatMetrics.RequestCount > 0 {
        averageDuration = formatMetrics.TotalDuration / time.Duration(formatMetrics.RequestCount)
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "request_count":          formatMetrics.RequestCount,
        "error_count":            formatMetrics.ErrorCount,
        "total_duration_ms":      formatMetrics.TotalDuration.Milliseconds(),
        "average_duration_ms":    averageDuration.Milliseconds(),
        "max_payload_size_bytes": formatMetrics.MaxPayloadSize,
    })
}

func main() {
    // Get the PORT from the environment variable, or fallback to 8030
    port := os.Getenv("PORT")
    if port == "" {
        port = "8030" // Default port
    }

    // Create a new router using Gorilla Mux
    r := mux.NewRouter()

    // Add logging middleware
    r.Use(Logger)

    // Define the endpoint for formatting with metrics middleware
    r.HandleFunc("/format", formatHandler).Methods("POST").Handler(FormatMetricsMiddleware(http.HandlerFunc(formatHandler)))

    // Define the metrics endpoint
    r.HandleFunc("/metrics", MetricsHandler).Methods("GET")

    // Start the HTTP server
    log.Printf("Server is running on http://localhost:%s", port)
    if err := http.ListenAndServe(":"+port, r); err != nil {
        log.Fatalf("Server failed to start: %s", err)
    }
}