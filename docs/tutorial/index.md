# Tutorial: Build a Meal Planner

Learn Conductor by building something genuinely useful: an automated meal planning system that generates weekly meal plans based on what's in your pantry.

## What You'll Build

By the end of this tutorial, you'll have a workflow that:
- Generates personalized meal suggestions
- Runs three LLM calls in parallel for speed
- Iterates until quality meets your standards
- Runs automatically on a schedule
- Reads your actual pantry inventory
- Delivers results to your preferred destination

## What You'll Learn

| Part | Feature | Concept |
|------|---------|---------|
| 1 | [Your First Workflow](first-workflow) | LLM steps, file output |
| 2 | [Parallel Execution](parallel) | Concurrent steps, performance |
| 3 | [Refinement Loops](loops) | Iteration until quality |
| 4 | [Scheduled Triggers](triggers) | Automation with cron |
| 5 | [Reading External Data](input) | File input, dynamic prompts |
| 6 | [Delivering Results](output) | Output to external services |
| 7 | [Deploy to Production](deploy) | Always-on server deployment |

## Prerequisites

Before starting:
1. **Conductor installed** - Run `conductor --version` to verify
2. **LLM provider configured** - Complete the [Hello World](../getting-started/hello-world) test
3. **About 90 minutes** - Each part takes 10-15 minutes

## The Pattern

Each tutorial part follows the same structure:
1. **What you'll learn** - New features introduced
2. **The workflow** - Complete, runnable YAML
3. **How it works** - Step-by-step explanation
4. **Try it** - Run and experiment
5. **Pattern spotlight** - Apply this elsewhere

## Why Meal Planning?

We chose meal planning because:
- **Relatable** - Everyone eats, everyone plans meals
- **Progressive** - Naturally builds from simple to sophisticated
- **Useful** - You might actually use this
- **Integrations** - Has clear inputs (pantry) and outputs (plan, shopping list)

Feel free to adapt the prompts to your dietary preferences, cuisine interests, or household size as you go.

## Start Building

Ready? Let's create your first workflow:

[Part 1: Your First Workflow →](first-workflow)
