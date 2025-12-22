#!/bin/bash
set -e

# Deploy docs to GitHub Pages (gh-pages branch)
# Usage: ./scripts/deploy-docs.sh

cd "$(dirname "$0")/.."
ROOT=$(pwd)

echo "Building docs..."
cd docs-site
npm ci
npm run build

echo "Deploying to gh-pages..."
cd dist

# Initialize git in dist folder
git init
git checkout -b gh-pages
git add -A
git commit -m "Deploy docs $(date '+%Y-%m-%d %H:%M:%S')"

# Force push to gh-pages branch
git push -f git@github.com:tombee/conductor.git gh-pages

echo "Done! Site will be available at https://tombee.github.io/conductor/"
