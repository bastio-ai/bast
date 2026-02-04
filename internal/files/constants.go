package files

const (
	// MaxSingleFileBytes is the maximum size of a single file that can be read (50KB)
	MaxSingleFileBytes = 50 * 1024

	// MaxTotalFileBytes is the maximum total bytes to read across all files (100KB)
	MaxTotalFileBytes = 100 * 1024

	// MaxSearchDepth is the maximum directory depth for file searches
	MaxSearchDepth = 5

	// MaxSuggestions is the default maximum number of file suggestions
	MaxSuggestions = 10
)
