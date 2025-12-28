# Write Song Example

Generate original songs with lyrics and chord symbols in various musical genres and keys.

## Description

This workflow creates complete songs with genre-appropriate structures, chord progressions, and lyrical styles. The AI selects authentic song forms (12-bar blues, verse-chorus, AABA) and uses chords diatonic to the specified musical key.

## Use Cases

- **Creative inspiration** - Generate song ideas and structures to build upon
- **Educational examples** - Demonstrate song structure and theory concepts
- **Placeholder content** - Create sample songs for music apps or services
- **Rapid prototyping** - Quickly sketch out musical ideas with proper structure

## Prerequisites

### Required

- Conductor installed ([Getting Started](../getting-started/))
- LLM provider configured (Claude Code, Anthropic API, or OpenAI)

### Optional

- Musical knowledge helpful but not required
- Text-to-speech or music notation software to hear/visualize output

## How to Run It

### Interactive Mode

Run without inputs to be prompted for each value:

```bash
conductor run examples/write-song

# Prompts appear:
genre: blues
topic: morning coffee
key (C Major):  # Press Enter for default or specify "G Major"
```

### Command Line Arguments

Provide all inputs directly:

```bash
# Blues song in C Major
conductor run examples/write-song \
  -i genre="blues" \
  -i topic="morning coffee" \
  -i key="C Major"

# Folk song in G Major
conductor run examples/write-song \
  -i genre="folk" \
  -i topic="coming home" \
  -i key="G Major"

# Jazz song in A minor
conductor run examples/write-song \
  -i genre="jazz" \
  -i topic="city nights" \
  -i key="A minor"
```

### Save to File

Redirect output to save the song:

```bash
conductor run examples/write-song \
  -i genre="country" \
  -i topic="dusty roads" \
  -i key="D Major" > my-song.txt
```

## Code Walkthrough

This is the simplest example workflow—a single LLM step with carefully crafted prompts:

### Single-Step Composition

```yaml
- id: compose
  name: Compose Song
  type: llm
  model: balanced
  prompt: |
    Write a short {{.inputs.genre}} song about "{{.inputs.topic}}" in the key of {{.inputs.key}}.

    Use a song structure authentic to {{.inputs.genre}} (e.g., 12-bar blues, verse-chorus,
    AABA, etc.). Include chord symbols above the lyrics, using chords diatonic to
    {{.inputs.key}} with progressions that fit the style.
```

**What it does**: Generates a complete song in a single step by combining genre knowledge, music theory (diatonic chords), and lyrical creativity.

**Model tier choice**: Uses `balanced` tier because song composition requires:
- **Creative writing** for lyrics
- **Music theory knowledge** for chord progressions
- **Genre understanding** for authentic structures

The `fast` tier would produce correct but less nuanced results. The `strategic` tier would be overkill for this creative task.

**Template variables in action**: The prompt dynamically incorporates user inputs (`{{.inputs.genre}}`, `{{.inputs.topic}}`, `{{.inputs.key}}`) to create a customized song. This single prompt template works for any genre/topic/key combination.

**Implicit structure guidance**: By mentioning "song structure authentic to {{.inputs.genre}}", the prompt leverages the model's training on musical patterns without needing to explicitly define every genre's structure.

## Genre Characteristics

Different genres produce distinct song structures and styles:

### Blues

**Structure**: Typically 12-bar blues (AAB lyric pattern)
**Chords**: Dominant 7th chords (C7, F7, G7)
**Progression**: I7-IV7-I7-V7-IV7-I7

Example:
```
C7                                F7
Woke up this morning, coffee on my mind    (A)
F7                        C7
Woke up this morning, coffee on my mind    (A)
G7                 F7                C7
Can't start my day without that grind      (B)
```

### Folk

**Structure**: Verse-chorus with simple progressions
**Chords**: Major and minor triads
**Progression**: I-V-vi-IV or I-IV-V

Example:
```
Verse:
C              G
Dusty road ahead
Am           F
Miles from home

Chorus:
C        G        Am      F
Walking home, one step at a time
```

### Country

**Structure**: Storytelling verses with singable choruses
**Chords**: I-IV-V with occasional ii or vi
**Style**: Narrative lyrics, often with a twist or moral

### Jazz

**Structure**: AABA (32-bar standard form)
**Chords**: Extended chords (maj7, min7, dom7, dim7)
**Progression**: ii-V-I, circle of fifths

Example:
```
Cmaj7    Am7      Dm7     G7        (A)
City lights are glowing in the rain
Cmaj7    Am7      Dm7     G7        (A)
Saxophone echoes down the lane
Fmaj7           Em7    A7           (B - Bridge)
Midnight jazz in a smoky room
Cmaj7    Am7      Dm7     G7        (A)
Dancing shadows beneath the moon
```

### Rock

**Structure**: Verse-chorus-bridge
**Chords**: Power chords, I-IV-V progressions
**Style**: Driving rhythm, repetitive hooks

## Customization Options

### 1. Specify Song Length

Add length constraints to the prompt:

```yaml
prompt: |
  Write a {{.inputs.length}} {{.inputs.genre}} song...
  (short: 2 verses + chorus, medium: 3 verses + chorus + bridge, long: full song)
```

### 2. Add Tempo and Feel

Include musical direction:

```yaml
inputs:
  - name: tempo
    type: string
    default: "moderate"
    description: "Tempo (slow, moderate, fast)"
  - name: feel
    type: string
    default: "straight"
    description: "Feel (straight, swing, shuffle)"

prompt: |
  Write a {{.inputs.tempo}}, {{.inputs.feel}} {{.inputs.genre}} song...
```

### 3. Request Specific Themes

Guide lyrical content:

```yaml
inputs:
  - name: mood
    type: string
    description: "Mood (happy, melancholic, uplifting, dark)"

prompt: |
  Write a {{.inputs.mood}} {{.inputs.genre}} song about "{{.inputs.topic}}"...
```

### 4. Output in Different Formats

Request various output formats:

```yaml
# Chord chart format
prompt: |
  ...
  Format as a chord chart with sections clearly labeled:
  [Verse 1]
  [Chorus]
  [Bridge]

# LeadSheet format (more musical notation)
prompt: |
  ...
  Format as a lead sheet with:
  - Section markers
  - Chord symbols above lyrics
  - Repeat marks and navigation
  - Suggested rhythm feel
```

### 5. Multi-Step Song Development

Expand into a workflow with multiple steps:

```yaml
steps:
  - id: structure
    type: llm
    model: fast
    prompt: "Outline song structure for {{.inputs.genre}}"

  - id: chord_progression
    type: llm
    model: fast
    prompt: "Create chord progression in {{.inputs.key}} for: {{.steps.structure.response}}"

  - id: lyrics
    type: llm
    model: balanced
    prompt: "Write lyrics about {{.inputs.topic}} fitting: {{.steps.structure.response}}"

  - id: combine
    type: llm
    model: fast
    prompt: "Combine chords and lyrics into final song"
```

## Example Outputs

### Blues in E Major
```
E7
Woke up this morning, blues on my mind
A7                           E7
Woke up this morning, blues on my mind
B7                    A7                E7
Can't shake this feeling, Lord knows I've tried

E7
My baby left me, took the morning train
A7                                E7
My baby left me, took the morning train
B7                 A7               E7
Nothing left but memories and this pain
```

### Folk in G Major
```
[Verse 1]
G              D
Fields of gold and green
Em           C
Where I was young
G              D
Calling me back home
Em        C       G
To where I belong

[Chorus]
G         D         Em      C
Coming home, coming home at last
G         D              Em        C
Leaving all my worries in the past
```

### Jazz in Bb Major
```
[A]
Bbmaj7    Gm7       Cm7      F7
City nights beneath the neon glow
Bbmaj7    Gm7       Cm7      F7
Jazz club where the cool cats go

[A]
Bbmaj7    Gm7       Cm7      F7
Trumpet sings a bittersweet refrain
Bbmaj7    Gm7       Cm7      F7
Washing all my troubles down the drain

[B]
Ebmaj7           Dm7    G7
In this moment, time stands still
Cmaj7            Cm7    F7
Piano player bending to his will

[A]
Bbmaj7    Gm7       Cm7      F7
City nights, they'll never let me go
```

## Common Issues and Solutions

### Issue: Chords don't match the key

**Symptom**: Output includes chords not in the specified key

**Solution**: Be more explicit about diatonic chords:

```yaml
prompt: |
  Write a song in {{.inputs.key}}.

  IMPORTANT: Only use chords diatonic to {{.inputs.key}}:
  - Major keys: I, ii, iii, IV, V, vi, vii°
  - Minor keys: i, ii°, III, iv, v, VI, VII

  For example, in C Major: C, Dm, Em, F, G, Am, Bdim
```

### Issue: Song structure doesn't match genre

**Symptom**: Blues song uses verse-chorus instead of 12-bar blues

**Solution**: Be specific about expected structure:

```yaml
prompt: |
  Write a {{.inputs.genre}} song using the AUTHENTIC structure:

  Blues: 12-bar blues with AAB lyric pattern
  Folk: Verse-chorus form
  Jazz: 32-bar AABA form
  Country: Verse-chorus with narrative verses
  Rock: Verse-chorus-bridge
```

### Issue: Output is too short or too long

**Symptom**: Song is just one verse or goes on too long

**Solution**: Specify length explicitly:

```yaml
prompt: |
  Write a SHORT {{.inputs.genre}} song (2-3 verses maximum)...
```

Or request specific section counts:

```yaml
prompt: |
  Include:
  - 2 verses
  - 1 chorus (repeated after each verse)
  - 1 bridge (before final chorus)
```

### Issue: Lyrics don't fit the rhythm

**Symptom**: Syllable count doesn't match chord changes

**Solution**: Request rhythmic consistency:

```yaml
prompt: |
  Ensure consistent syllable counts per line so lyrics fit the rhythm.
  Each line in a verse should have similar syllable counts.
```

## Related Examples

This is a simple, single-step creative workflow. For more complex patterns, see:

- [Issue Triage](issue-triage.md) - Sequential multi-step analysis
- [Code Review](code-review.md) - Parallel execution pattern
- [Slack Integration](slack-integration.md) - Formatting and output patterns

## Workflow Files

Full workflow definition: [examples/write-song/workflow.yaml](https://github.com/tombee/conductor/blob/main/examples/write-song/workflow.yaml)

## Further Reading

- [Template Variables](../reference/cheatsheet.md#template-variables)
- [LLM Step Configuration](../reference/workflow-schema.md#llm-step)
- [Model Tiers](../architecture/llm-providers.md#model-tiers)
- [Building Workflows](../building-workflows/patterns.md)
