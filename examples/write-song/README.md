# Write Song

Generate songs with lyrics and chord symbols. The LLM picks an authentic song structure for the genre and uses chords diatonic to the specified key.

## Usage

```bash
$ conductor run examples/write-song/workflow.yaml

genre: blues
topic: morning coffee
key (C Major):
```

Conductor prompts for required inputs. Press Enter to accept the default key (C Major), or specify a different one:

```bash
genre: folk
topic: coming home
key (C Major): G Major
```

## Inputs

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| genre | string | yes | - | Musical genre (blues, folk, country, rock, jazz, etc.) |
| topic | string | yes | - | What the song is about |
| key | string | no | C Major | Musical key (C Major, G Major, A minor, etc.) |

## How It Works

The LLM:
1. Picks a song structure authentic to the genre (12-bar blues, verse-chorus, AABA, etc.)
2. Uses chords diatonic to the specified key
3. Writes genre-appropriate progressions (I-IV-V for blues, I-V-vi-IV for pop, etc.)
4. Formats with chord symbols above lyrics

## Example Genres

- **blues** - 12-bar blues with AAB lyric structure, dominant 7th chords
- **folk** - Verse-chorus with simple progressions, often in major keys
- **country** - Storytelling verses, singable choruses, I-IV-V progressions
- **jazz** - AABA form, extended chords (maj7, min7, dom7)
- **rock** - Power chord progressions, verse-chorus-bridge structure
