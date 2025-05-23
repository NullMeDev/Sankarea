// cmd/sankarea/embed.go
package main

import (
    "embed"
    "encoding/json"
    "fmt"
    "html/template"
    "io/fs"
    "net/http"
    "path"
    "strings"
)

//go:embed templates/* static/*
var embedFS embed.FS

// Asset types that need special handling
const (
    contentTypeJSON = "application/json"
    contentTypeCSS  = "text/css"
    contentTypeJS   = "application/javascript"
    contentTypeHTML = "text/html"
)

// Template functions map
var templateFuncs = template.FuncMap{
    "formatTime": formatTime,
    "formatDuration": FormatDuration,
    "safeHTML": func(s string) template.HTML {
        return template.HTML(s)
    },
    "getConfig": func() interface{} {
        return GetConfig()
    },
    "getState": func() interface{} {
        return GetState()
    },
}

// EmbeddedAssets handles serving embedded static files and templates
type EmbeddedAssets struct {
    templates  *template.Template
    staticFS   fs.FS
    staticDir  string
}

// NewEmbeddedAssets creates a new embedded assets handler
func NewEmbeddedAssets() (*EmbeddedAssets, error) {
    // Initialize templates
    templates, err := parseTemplates()
    if err != nil {
        return nil, fmt.Errorf("failed to parse templates: %v", err)
    }

    // Set up static files
    staticFS, err := fs.Sub(embedFS, "static")
    if err != nil {
        return nil, fmt.Errorf("failed to set up static files: %v", err)
    }

    return &EmbeddedAssets{
        templates: templates,
        staticFS:  staticFS,
        staticDir: "static",
    }, nil
}

// parseTemplates loads and parses all embedded templates
func parseTemplates() (*template.Template, error) {
    templates := template.New("").Funcs(templateFuncs)

    // Read all template files
    entries, err := embedFS.ReadDir("templates")
    if err != nil {
        return nil, fmt.Errorf("failed to read templates directory: %v", err)
    }

    // Parse each template file
    for _, entry := range entries {
        if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
            templatePath := path.Join("templates", entry.Name())
            content, err := embedFS.ReadFile(templatePath)
            if err != nil {
                return nil, fmt.Errorf("failed to read template %s: %v", templatePath, err)
            }

            if _, err := templates.New(entry.Name()).Parse(string(content)); err != nil {
                return nil, fmt.Errorf("failed to parse template %s: %v", templatePath, err)
            }
        }
    }

    return templates, nil
}

// ServeStatic returns an http.Handler for serving static files
func (ea *EmbeddedAssets) ServeStatic() http.Handler {
    return http.FileServer(http.FS(ea.staticFS))
}

// RenderTemplate renders a template with the given data
func (ea *EmbeddedAssets) RenderTemplate(w http.ResponseWriter, name string, data interface{}) error {
    // Add common data to all templates
    commonData := map[string]interface{}{
        "Version":     cfg.Version,
        "State":       GetState(),
        "Config":      GetConfig(),
        "LastMetrics": GetLastMetrics(),
    }

    // Merge with template-specific data
    var templateData map[string]interface{}
    if data != nil {
        switch v := data.(type) {
        case map[string]interface{}:
            templateData = v
        default:
            // Convert data to map using JSON marshaling
            b, err := json.Marshal(data)
            if err != nil {
                return fmt.Errorf("failed to marshal template data: %v", err)
            }
            if err := json.Unmarshal(b, &templateData); err != nil {
                return fmt.Errorf("failed to unmarshal template data: %v", err)
            }
        }
    } else {
        templateData = make(map[string]interface{})
    }

    // Merge common data
    for k, v := range commonData {
        if _, exists := templateData[k]; !exists {
            templateData[k] = v
        }
    }

    // Set content type
    w.Header().Set("Content-Type", contentTypeHTML)

    // Execute template
    if err := ea.templates.ExecuteTemplate(w, name, templateData); err != nil {
        return fmt.Errorf("failed to execute template %s: %v", name, err)
    }

    return nil
}

// GetStaticFile retrieves a static file's contents
func (ea *EmbeddedAssets) GetStaticFile(filepath string) ([]byte, string, error) {
    // Clean the file path
    filepath = path.Clean(filepath)
    if !strings.HasPrefix(filepath, ea.staticDir) {
        filepath = path.Join(ea.staticDir, filepath)
    }

    // Read the file
    content, err := embedFS.ReadFile(filepath)
    if err != nil {
        return nil, "", fmt.Errorf("failed to read static file %s: %v", filepath, err)
    }

    // Determine content type
    contentType := getContentType(filepath)

    return content, contentType, nil
}

// getContentType determines the content type based on file extension
func getContentType(filepath string) string {
    switch {
    case strings.HasSuffix(filepath, ".css"):
        return contentTypeCSS
    case strings.HasSuffix(filepath, ".js"):
        return contentTypeJS
    case strings.HasSuffix(filepath, ".json"):
        return contentTypeJSON
    case strings.HasSuffix(filepath, ".html"):
        return contentTypeHTML
    default:
        return http.DetectContentType([]byte(filepath))
    }
}

// ReloadTemplates reloads all templates
func (ea *EmbeddedAssets) ReloadTemplates() error {
    templates, err := parseTemplates()
    if err != nil {
        return fmt.Errorf("failed to reload templates: %v", err)
    }

    ea.templates = templates
    return nil
}
