#!/bin/bash
# Copyright 2025 Tom Barlow
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Lint help text in Conductor CLI commands
# Validates:
# - Example count >= 3 per command
# - Short description < 50 chars
# - No API key patterns (AKIA..., sk-...)
# - No email addresses (except @example.com)
# - No real IPs (except 192.0.2.x TEST-NET-1)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

echo "Linting CLI help text..."
echo ""

# Find all command files
COMMAND_FILES=$(find "$PROJECT_ROOT/internal/commands" -name "*.go" -type f | grep -v "_test.go" | sort)

for file in $COMMAND_FILES; do
    # Extract command Short field
    SHORT=$(grep -A 1 'Short:' "$file" | grep '"' | head -1 | sed 's/.*"\(.*\)".*/\1/' | tr -d '\n' || true)

    if [ -n "$SHORT" ]; then
        SHORT_LEN=${#SHORT}

        # Check short description length
        if [ $SHORT_LEN -ge 50 ]; then
            echo -e "${RED}ERROR${NC}: $file"
            echo "  Short description too long ($SHORT_LEN chars, max 50): $SHORT"
            ((ERRORS++))
        fi
    fi

    # Count examples in Example field
    EXAMPLE_COUNT=$(grep -A 100 'Example:' "$file" | grep -c '# Example [0-9]' || true)

    # Only check if we found an Example field
    if grep -q 'Example:' "$file"; then
        if [ $EXAMPLE_COUNT -lt 3 ]; then
            # Skip root command and certain special commands that may not need 3 examples
            BASENAME=$(basename "$file")
            if [[ "$BASENAME" != "root.go" ]] && [[ "$BASENAME" != "version.go" ]]; then
                echo -e "${YELLOW}WARNING${NC}: $file"
                echo "  Insufficient examples (found $EXAMPLE_COUNT, expected 3+)"
                ((WARNINGS++))
            fi
        fi
    fi

    # Check for API key patterns
    if grep -E 'AKIA[0-9A-Z]{16}|sk-[a-zA-Z0-9]{32,}' "$file" > /dev/null 2>&1; then
        # Check if it's in a comment explaining what NOT to do
        if ! grep -B 2 -A 2 'AKIA\|sk-' "$file" | grep -qi 'do not\|avoid\|never\|example of bad'; then
            echo -e "${RED}ERROR${NC}: $file"
            echo "  Found potential API key pattern"
            ((ERRORS++))
        fi
    fi

    # Check for email addresses (except @example.com, @anthropic.com)
    if grep -E '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}' "$file" | grep -v '@example.com' | grep -v '@anthropic.com' > /dev/null 2>&1; then
        echo -e "${RED}ERROR${NC}: $file"
        echo "  Found real email address (use @example.com instead)"
        ((ERRORS++))
    fi

    # Check for real IPs (except 192.0.2.x TEST-NET-1, 127.0.0.1, 0.0.0.0)
    if grep -oE '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' "$file" | grep -v '^192\.0\.2\.' | grep -v '^127\.0\.0\.1$' | grep -v '^0\.0\.0\.0$' | grep -v '^255\.255\.255\.' > /dev/null 2>&1; then
        # Check if it's in a comment or string context
        MATCHES=$(grep -n -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' "$file" | grep -v '192\.0\.2\.' | grep -v '127\.0\.0\.1' | grep -v '0\.0\.0\.0' || true)
        if [ -n "$MATCHES" ]; then
            # Only warn if it looks like it's in an example, not in a URL or comment
            if echo "$MATCHES" | grep -E 'Example:|example' > /dev/null 2>&1; then
                echo -e "${YELLOW}WARNING${NC}: $file"
                echo "  Found potential real IP address (use 192.0.2.x TEST-NET-1 instead)"
                ((WARNINGS++))
            fi
        fi
    fi
done

echo ""
if [ $ERRORS -gt 0 ]; then
    echo -e "${RED}✗ Found $ERRORS error(s) and $WARNINGS warning(s)${NC}"
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    echo -e "${YELLOW}⚠ Found $WARNINGS warning(s)${NC}"
    exit 0
else
    echo -e "${GREEN}✓ All help text checks passed${NC}"
    exit 0
fi
