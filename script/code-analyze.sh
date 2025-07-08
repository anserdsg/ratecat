#!/bin/bash

# code-analyze.sh - Code analysis tool for Go repositories
# Usage: ./build/code-analyze.sh <command> <repo_folder_path>
# Commands: count, summary, report, testcase

set -e

# Function to display usage
show_usage() {
    cat << EOF
Usage: $0 <command> <repo_folder_path>

Commands:
  count      - Detailed code line count analysis
  summary    - Machine-readable summary for CI
  report     - Comprehensive report with raw and effective lines
  testcase   - Count test cases in repository

Examples:
  $0 count /path/to/repo
  $0 summary .
  $0 testcase- ../my-project
EOF
}

# Check arguments
if [ $# -ne 2 ]; then
    echo "Error: Invalid number of arguments"
    show_usage
    exit 1
fi

COMMAND="$1"
REPO_PATH="$2"

# Validate repository path
if [ ! -d "$REPO_PATH" ]; then
    echo "Error: Repository path '$REPO_PATH' does not exist or is not a directory"
    exit 1
fi

# Convert to absolute path
REPO_PATH=$(cd "$REPO_PATH" && pwd)

# Change to repository directory
cd "$REPO_PATH" || exit 1

# Check if it's a Go repository
if [ ! -f "go.mod" ]; then
    echo "Warning: No go.mod found in '$REPO_PATH'. This may not be a Go repository."
fi

# Debug function to check if files exist
debug_files() {
    echo "Debug: Current directory: $(pwd)"
    echo "Debug: Go files found:"
    find . -type f -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | head -5
    echo "Debug: Total Go files: $(find . -type f -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | wc -l)"
}

# Get test directories
get_test_dirs() {
    find . -name "*_test.go" -type f | sed 's|/[^/]*$||' | sort -u
}

# Code line count with detailed breakdown
code_line_count() {
    printf "Calculating code line counts (including blank lines)...\n"
    printf "%s\n" "$(printf '%.0s-' {1..70})"

    total=$(find . -type f -name "*.go" \
        -not -path "*/vendor/*" \
        -not -path "*/.git/*" \
        -not -path "*/node_modules/*" \
        2>/dev/null | wc -l)

    if [ "$total" -eq "0" ]; then
        printf "Warning: No Go files found in current directory: $(pwd)\n"
        printf "Files in current directory:\n"
        ls -la | head -10
        return
    fi

    # Count lines for each file type
    total_lines=0
    non_test_lines=0
    test_lines=0
    mock_lines=0
    proto_lines=0

    # Count total lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        total_lines=$((total_lines + lines))
    done < <(find . -type f -name "*.go" \
        -not -path "*/vendor/*" \
        -not -path "*/.git/*" \
        -not -path "*/node_modules/*" \
        -print0 2>/dev/null)

    # Count production code lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        non_test_lines=$((non_test_lines + lines))
    done < <(find . -type f -name "*.go" \
        -not -name "*_test.go" \
        -not -name "*mock*.go" \
        -not -name "*.pb.go" \
        -not -path "*/vendor/*" \
        -not -path "*/.git/*" \
        -not -path "*/node_modules/*" \
        -print0 2>/dev/null)

    # Count test lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        test_lines=$((test_lines + lines))
    done < <(find . -type f -name "*_test.go" \
        -not -path "*/vendor/*" \
        -not -path "*/.git/*" \
        -print0 2>/dev/null)

    # Count mock lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        mock_lines=$((mock_lines + lines))
    done < <(find . -type f -name "*mock*.go" \
        -not -path "*/vendor/*" \
        -not -path "*/.git/*" \
        -print0 2>/dev/null)

    # Count proto lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        proto_lines=$((proto_lines + lines))
    done < <(find . -type f -name "*.pb.go" \
        -not -path "*/vendor/*" \
        -not -path "*/.git/*" \
        -print0 2>/dev/null)

    if [ "$total_lines" -eq "0" ]; then
        printf "Warning: No Go files found or all files are empty.\n"
    else
        printf "%-30s: %6d lines\n" "Total lines (all)" "$total_lines"

        if [ "$total_lines" -gt "0" ]; then
            printf "%-30s: %6d lines (%5.2f%%)\n" "Production code" "$non_test_lines" \
                $(awk "BEGIN {printf \"%.2f\", ($non_test_lines / $total_lines) * 100}")
            printf "%-30s: %6d lines (%5.2f%%)\n" "Test code" "$test_lines" \
                $(awk "BEGIN {printf \"%.2f\", ($test_lines / $total_lines) * 100}")
        fi

        if [ "$mock_lines" -gt "0" ]; then
            printf "%-30s: %6d lines (%5.2f%%)\n" "Mock/Generated code" "$mock_lines" \
                $(awk "BEGIN {printf \"%.2f\", ($mock_lines / $total_lines) * 100}")
        fi

        if [ "$proto_lines" -gt "0" ]; then
            printf "%-30s: %6d lines (%5.2f%%)\n" "Protobuf generated code" "$proto_lines" \
                $(awk "BEGIN {printf \"%.2f\", ($proto_lines / $total_lines) * 100}")
        fi

        printf "%s\n" "$(printf '%.0s-' {1..70})"
        if [ "$non_test_lines" -gt "0" ]; then
            test_ratio=$(awk "BEGIN {printf \"%.2f\", $test_lines / $non_test_lines}")
            printf "%-30s: %6s:1 (%.0f test lines per 100 production lines)\n" \
                "Test-to-production ratio" "$test_ratio" \
                $(awk "BEGIN {printf \"%.0f\", $test_ratio * 100}")
        fi
    fi
}

# Machine-readable summary
code_line_summary() {
    # Use the same counting approach as above but simpler
    total_lines=0
    non_test_lines=0
    test_lines=0

    # Count total lines excluding mock and proto
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        total_lines=$((total_lines + lines))
    done < <(find . -type f -name "*.go" \
        -not -path "*/vendor/*" \
        -not -name "*mock*.go" \
        -not -name "*.pb.go" \
        -print0 2>/dev/null)

    # Count production code lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        non_test_lines=$((non_test_lines + lines))
    done < <(find . -type f -name "*.go" \
        -not -name "*_test.go" \
        -not -name "*mock*.go" \
        -not -name "*.pb.go" \
        -not -path "*/vendor/*" \
        -print0 2>/dev/null)

    # Count test lines
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        test_lines=$((test_lines + lines))
    done < <(find . -type f -name "*_test.go" \
        -not -path "*/vendor/*" \
        -print0 2>/dev/null)

    if [ "$non_test_lines" -gt "0" ]; then
        test_ratio=$(awk "BEGIN {printf \"%.2f\", $test_lines / $non_test_lines}")
    else
        test_ratio="0.00"
    fi
	printf "Machine-readable summary(excluding mock files):\n"
    printf "TOTAL=%d PROD=%d TEST=%d RATIO=%s\n" "$total_lines" "$non_test_lines" "$test_lines" "$test_ratio"
}

# Comprehensive report with both raw and effective lines
code_line_report() {
    printf "Complete code line analysis...\n"
    printf "%s\n" "$(printf '%.0s=' {1..80})"
    printf "Raw Line Counts (including blank lines and comments):\n"
    printf "%s\n" "$(printf '%.0s-' {1..80})"

    # Use the same counting approach as code_line_count for raw counts
    total_raw=0
    prod_raw=0
    test_raw=0

    # Raw counts using the same approach
    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        total_raw=$((total_raw + lines))
    done < <(find . -type f -name "*.go" \
        -not -path "*/vendor/*" -not -path "*/.git/*" \
        -print0 2>/dev/null)

    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        prod_raw=$((prod_raw + lines))
    done < <(find . -type f -name "*.go" \
        -not -name "*_test.go" -not -name "*mock*.go" -not -name "*.pb.go" \
        -not -path "*/vendor/*" -not -path "*/.git/*" \
        -print0 2>/dev/null)

    while IFS= read -r -d '' file; do
        lines=$(wc -l < "$file" 2>/dev/null || echo 0)
        test_raw=$((test_raw + lines))
    done < <(find . -type f -name "*_test.go" \
        -not -path "*/vendor/*" -not -path "*/.git/*" \
        -print0 2>/dev/null)

    # Effective counts (excluding blank lines and comments)
    total_clean=$(find . -type f -name "*.go" \
        -not -path "*/vendor/*" -not -path "*/.git/*" \
        -exec cat {} \; 2>/dev/null | grep -v '^[[:space:]]*$' | grep -v '^[[:space:]]*//' | wc -l)

    prod_clean=$(find . -type f -name "*.go" \
        -not -name "*_test.go" -not -name "*mock*.go" -not -name "*.pb.go" \
        -not -path "*/vendor/*" -not -path "*/.git/*" \
        -exec cat {} \; 2>/dev/null | grep -v '^[[:space:]]*$' | grep -v '^[[:space:]]*//' | wc -l)

    test_clean=$(find . -type f -name "*_test.go" \
        -not -path "*/vendor/*" -not -path "*/.git/*" \
        -exec cat {} \; 2>/dev/null | grep -v '^[[:space:]]*$' | grep -v '^[[:space:]]*//' | wc -l)

    printf "%-30s: %6d lines\n" "Total (all files)" "$total_raw"
    printf "%-30s: %6d lines\n" "Production code" "$prod_raw"
    printf "%-30s: %6d lines\n" "Test code" "$test_raw"
    printf "\n"

    printf "Effective Lines (excluding blank lines and comments):\n"
    printf "%s\n" "$(printf '%.0s-' {1..80})"
    printf "%-30s: %6d lines\n" "Total (effective)" "$total_clean"
    printf "%-30s: %6d lines\n" "Production code" "$prod_clean"
    printf "%-30s: %6d lines\n" "Test code" "$test_clean"
    printf "\n"

    printf "Code Density Analysis:\n"
    printf "%s\n" "$(printf '%.0s-' {1..80})"
    if [ "$total_raw" -gt "0" ]; then
        density=$(awk "BEGIN {printf \"%.2f\", ($total_clean / $total_raw) * 100}")
        printf "%-30s: %6.2f%%\n" "Code density" "$density"
    fi
    if [ "$prod_clean" -gt "0" ]; then
        test_ratio=$(awk "BEGIN {printf \"%.2f\", $test_clean / $prod_clean}")
        printf "%-30s: %6.2f:1\n" "Test-to-production ratio" "$test_ratio"
    fi
}

# Count test cases
testcase_count() {
    printf "Counting test cases in Go repository...\n"
    printf "%s\n" "$(printf '%.0s-' {1..60})"

    total=0
    test_dirs=$(get_test_dirs)

    for dir in $test_dirs; do
        if [ -d "$dir" ]; then
            count=$(find "$dir" -maxdepth 1 -name "*_test.go" -type f -print0 2>/dev/null | \
                xargs -0 grep -h "^func \(Test\|Benchmark\|Example\)" 2>/dev/null | \
                wc -l)

            if [ "$count" -gt 0 ]; then
                printf "%-40s: %3d test cases\n" "$dir" "$count"
                total=$((total + count))
            fi
        fi
    done

    printf "%s\n" "$(printf '%.0s-' {1..60})"
    printf "%-40s: %3d test cases\n" "Total" "$total"
}

# Execute command
case "$COMMAND" in
    "count")
        code_line_count
        ;;
    "summary")
        code_line_summary
        ;;
    "report")
        code_line_report
        ;;
    "testcase")
        testcase_count
        ;;
    *)
        echo "Error: Unknown command '$COMMAND'"
        show_usage
        exit 1
        ;;
esac