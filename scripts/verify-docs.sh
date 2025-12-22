#!/bin/bash
# Documentation verification script for Phase 8
# Run this before marking Phase 8 complete

set -e

DOCS_DIR="docs"
BUILD_DIR="site"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "================================"
echo "Documentation Verification Script"
echo "================================"
echo ""

# Check if running from project root
if [ ! -d "$DOCS_DIR" ]; then
    echo -e "${RED}Error: Must run from project root${NC}"
    exit 1
fi

# Activate virtual environment if it exists
if [ -d ".venv" ]; then
    echo "Activating virtual environment..."
    source .venv/bin/activate
fi

# Test 1: Build verification
echo -e "${YELLOW}[1/6] Testing MkDocs build...${NC}"
if mkdocs build --clean > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Build succeeds${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    mkdocs build
    exit 1
fi

# Test 2: Count pages
echo -e "${YELLOW}[2/6] Verifying page count...${NC}"
PAGE_COUNT=$(find "$BUILD_DIR" -name "*.html" | wc -l | tr -d ' ')
echo -e "${GREEN}✓ Built $PAGE_COUNT HTML pages${NC}"

# Test 3: Search for broken internal links (basic check)
echo -e "${YELLOW}[3/6] Checking for placeholder URLs...${NC}"
PLACEHOLDER_COUNT=$(find "$DOCS_DIR" -name "*.md" -exec grep -l "](http://localhost\|](http://127.0.0.1\|](http://example.com" {} \; 2>/dev/null | wc -l | tr -d ' ')
if [ "$PLACEHOLDER_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ No placeholder URLs found${NC}"
else
    echo -e "${RED}✗ Found $PLACEHOLDER_COUNT files with placeholder URLs${NC}"
    find "$DOCS_DIR" -name "*.md" -exec grep -l "](http://localhost\|](http://127.0.0.1\|](http://example.com" {} \;
fi

# Test 4: Check for TODO/FIXME markers
echo -e "${YELLOW}[4/6] Checking for placeholder text...${NC}"
TODO_COUNT=$(grep -r "TODO\|FIXME\|\[Content here\]\|XXX" "$DOCS_DIR"/*.md "$DOCS_DIR"/**/*.md 2>/dev/null | wc -l | tr -d ' ')
if [ "$TODO_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ No placeholder text found${NC}"
else
    echo -e "${RED}✗ Found $TODO_COUNT placeholder markers${NC}"
    grep -rn "TODO\|FIXME\|\[Content here\]\|XXX" "$DOCS_DIR"/*.md "$DOCS_DIR"/**/*.md 2>/dev/null | head -10
fi

# Test 5: Verify navigation structure
echo -e "${YELLOW}[5/6] Verifying navigation structure...${NC}"
if [ -f "mkdocs.yml" ]; then
    NAV_PAGES=$(grep -c "\.md" mkdocs.yml)
    echo -e "${GREEN}✓ Navigation defines $NAV_PAGES pages${NC}"
else
    echo -e "${RED}✗ mkdocs.yml not found${NC}"
    exit 1
fi

# Test 6: Check for syntax highlighting languages
echo -e "${YELLOW}[6/6] Checking code blocks...${NC}"
YAML_BLOCKS=$(grep -r '```yaml' "$DOCS_DIR" | wc -l | tr -d ' ')
GO_BLOCKS=$(grep -r '```go' "$DOCS_DIR" | wc -l | tr -d ' ')
BASH_BLOCKS=$(grep -r '```bash' "$DOCS_DIR" | wc -l | tr -d ' ')
echo -e "${GREEN}✓ Found $YAML_BLOCKS YAML code blocks${NC}"
echo -e "${GREEN}✓ Found $GO_BLOCKS Go code blocks${NC}"
echo -e "${GREEN}✓ Found $BASH_BLOCKS Bash code blocks${NC}"

# Summary
echo ""
echo "================================"
echo "Automated Checks Complete"
echo "================================"
echo ""
echo "Manual verification still needed:"
echo "  - Local preview (mkdocs serve)"
echo "  - Accessibility audit (Chrome Lighthouse)"
echo "  - Performance test (Chrome Lighthouse)"
echo "  - Search functionality"
echo "  - Mobile responsiveness"
echo "  - Link verification (click through all pages)"
echo ""
echo "See PHASE8_LAUNCH_CHECKLIST.md for full checklist"
