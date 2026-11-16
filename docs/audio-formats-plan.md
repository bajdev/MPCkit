# MPCkit audio formats: work plan

Written 2026-11-14. **Not started.** The plan begins with a check on the device that only bajdev can do; no code PR starts before its results are recorded and the decisions below are confirmed.

The goal: a pad accepts every audio format the MPC Sample imports. Akai's user guide (Technical Specifications, page 65, as quoted by bajdev) lists:

`.wav` `.mp3` `.aif` / `.aiff` `.snd` `.s1s` `.s3s` `.flac` `.ogg`

Today MPCkit accepts uncompressed WAV only. The scope document lists "Other audio formats" under "2.0 or later".

## The main question: copy the file, or convert it?

Akai's list describes importing through the device's own browser. MPCkit does something else: it puts files straight into a project's data folder and writes the project file that points to them. Whether the device reads an MP3 or a FLAC from there is not known. For WAV it was tested (build plan, section 7) and it works.

Two ways to build this:

| | A. Copy the file as it is | B. Convert to WAV at export |
| --- | --- | --- |
| What the device gets | `kick.mp3` in the data folder | `kick.wav`, made by MPCkit from `kick.mp3` |
| Depends on the device loading non-WAV files from a project | yes, untested | no: the device only ever sees WAV, which is tested |
| Frame count for `sliceInfo.End` | must equal the device's own count. For MP3 that is uncertain: decoders differ on how much silence they drop at the start and end. | exact: MPCkit wrote the file |
| Decoders needed in Go | still needed for the preview where the webview cannot play a format, and to count MP3 and Ogg frames | needed |
| Size on the card | as small as the source | MP3 and Ogg grow about ten times; FLAC about two times |
| Export speed | a file copy | decoding, a few seconds for long files |

**The plan proposes B.** It removes both unknowns, and A saves less than it seems to, because Go has to decode the files in either case. The device check below can overturn this: if the device keeps an imported MP3 as an MP3 in its own projects, A becomes worth a second look for card space.

## What it adds, and what it costs

Read this before deciding to build it.

| Format | Reader | New Go modules | Risk |
| --- | --- | --- | --- |
| `.wav` | exists | none | none |
| `.aif`, `.aiff` | written here, about 200 lines | none | low: a simple, documented format |
| `.flac` | module | `github.com/mewkiz/flac` and the 3 small modules it needs (public domain and Apache 2.0; last release 2026-08) | low |
| `.mp3` | module | `github.com/hajimehoshi/go-mp3` (Apache 2.0; last release 2022) | medium: not maintained recently |
| `.ogg` | module | `github.com/jfreymuth/oggvorbis` and `vorbis` (MIT; last release 2023) | medium: same, and Ogg files that hold Opus audio would be refused |
| `.snd`, `.s1s`, `.s3s` | written here from real files | none | high: see below |

- All the modules are pure Go, checked on 2026-11-14: no C code, so the Windows and Mac builds do not change. They are the project's first dependencies beyond Wails.
- Every decoder reads files from strangers. The review's S3 finding (a crafted project crashes the app) applies again to each one. PR F1 puts all decoding behind one function that turns a panic into an error, and each format gets tests with cut-off and damaged files.
- `.snd`, `.s1s` and `.s3s` are, as far as I know, sample files from Akai's older samplers (the MPC2000 family, the S1000 and the S3000). I know of no published specification and no Go module. A reader has to be worked out from real files and community notes, and I have no such files. They are the last PR, and the plan proposes to build it only if bajdev has files to test with.
- Estimate for everything except the old Akai formats: about 600 lines of Go without tests, about 400 of tests, and under 30 lines of frontend.

## What is already there

- `internal/wav` reads a WAV's header (`ReadFile`, `Validate`) and decodes it to 32-bit floats (`DecodeFile`).
- `AssignSample` (`internal/project/pads.go`) reads the header, checks it, names the pool file after the source and sets `sliceInfo.End` to the frame count minus 1.
- Export (`internal/export`) copies each source file into the data folder under its pool name.
- The preview asks Go for `/audio/<slot>`. Go serves the file itself with its frame count and sample rate in two headers (review plan, PR 13), or all frames as floats with `?pcm=1` when the webview cannot decode the file.
- WAV is assumed by name in: the file dialog's filter (`dialogs.go`), the file-drop handler (`main.go`, which ignores anything but `.wav`), the `-pad` flag's help in `mpckit-cli`, two texts in `PadPanel.svelte` and two in `App.svelte`, the README and the scope document.
- Limits kept from the WAV rule: mono or stereo, at most 20 minutes.

## Device checks

### D1: what the device does itself (before any code)

This is the same method that produced `Demo02`: let the device do the job, then read its file.

1. Make one short file in each format from the test tone in `testdata/wav-formats` (`44k 16bit mono.wav`: one second, with a higher beep at the end). On the laptop, with ffmpeg:

   ```
   ffmpeg -i "44k 16bit mono.wav" tone.mp3
   ffmpeg -i "44k 16bit mono.wav" tone.aif
   ffmpeg -i "44k 16bit mono.wav" tone.flac
   ffmpeg -i "44k 16bit mono.wav" -c:a libvorbis tone.ogg
   ```

   Add any `.snd`, `.s1s` or `.s3s` files you have.
2. On the device, in a new project, load each file onto its own pad through the device's browser: WAV on A1 as the control, then the others.
3. Note for each pad: does it load, and does it play at the same pitch and length as A1, with the end beep?
4. Save the project as `DemoFormats` and copy the project file and its data folder into `testdata/device/`.

What the saved project answers:

| Question | Where to look |
| --- | --- |
| Did the device keep `tone.mp3`, or write a WAV? | the data folder, and `samples[].path` |
| If it wrote a WAV: what sample rate and bit depth? | the WAV's header |
| What frame count did it use? | `layer.sliceInfo.End`, against the 44,100 frames of the source |
| Does a non-WAV pool entry differ from a WAV one? | `samples[]`: `loadImpl`, `metadata` |

### D2: copying files as they are (only if approach A is still wanted after D1)

A hand-made project like the WAV test pack, with each non-WAV file placed in the data folder and named in the pool. Does it load, and does each pad play in full?

### D3: the finished feature (before the release)

A kit built in the app with one pad per format, exported and loaded on the device: every pad matches the WAV pad in pitch and length, with the end beep.

## Decisions

Proposed by the plan, **to confirm with bajdev after D1**:

| # | Topic | Proposal |
| --- | --- | --- |
| 1 | Copy or convert | Convert to WAV at export (approach B). |
| 2 | New Go modules | Approve the three decoders in the table above. Each arrives in its own PR, so one can be refused without stopping the others. |
| 3 | WAV written for a lossless source (AIFF, FLAC) | Same sample rate, channels and bit depth as the source. |
| 4 | WAV written for MP3 and Ogg | Same sample rate and channels, 16-bit. If D1 shows the device writing something else, follow the device. |
| 5 | Pool file name | The source's name with `.wav`: `kick.mp3` becomes `kick.wav`. A clash with an existing `kick.wav` is settled by the numbering already used for clashes. |
| 6 | Preview of a non-WAV pad | Always decoded by Go, never by the webview, so the preview plays exactly the frames the export will write. |
| 7 | How a format is recognised | By the file's first bytes, not its extension. A `.wav` that is really an MP3 is treated as an MP3. |
| 8 | WAV sources | Unchanged: still copied byte for byte, never rewritten. |
| 9 | Old Akai formats | Build PR F4 only if bajdev supplies test files. Otherwise the app says these formats are not supported yet. |
| 10 | Version | The next minor version after whatever is current when F1 starts (0.6.0 today). Still beta. |

Out of scope: converting sample rates, normalising volume, reading tags or tempo from files, compressed AIFF variants, Ogg files that hold Opus, and writing any format other than WAV.

## Guidelines for agents

Everything in "Guidelines for agents" in `docs/review-fixes-plan.md` applies. In addition:

- An agent that meets a choice this plan does not settle stops and asks bajdev in the PR record's Notes.
- A Go module may be added only in the PR this plan names it in, at the version current when the PR starts, after bajdev confirms decision 2. Run `govulncheck` with it in place and name the module and its licence in the PR record.
- Never decode a whole file into memory to learn its length. The largest allowed sample, 20 minutes of 96 kHz stereo, is over 900 MB as floats. Lengths come from headers where the format has them; export decodes in blocks.
- No decoder is called directly. Everything goes through `internal/audio`, which recovers a panic and returns it as an error naming the file.
- Refuse a file before decoding it if its header declares more than two channels or more than 20 minutes.
- Test files are made inside the tests where the format allows (WAV and AIFF are easy to write). For FLAC, MP3 and Ogg, commit small files made from the test tone, under `testdata/audio-formats/`, each under 50 kB, with a README saying how they were made.
- A WAV source's export stays byte-identical. `TestGoldenMpckitT1` and `TestExportMatchesCLIRecipe` must pass unchanged.
- If the waveform editor (`docs/waveform-plan.md`) is built too: its `SetTrim` must take the frame count from `internal/audio`, not from `wav.ReadFile`. Whichever plan goes second makes that change.

## PR order

```
D1 device check  →  decisions confirmed
  →  PR F1 audio-package (AIFF)  →  PR F2 flac-import  →  PR F3 mp3-ogg-import  →  [PR F4 akai-legacy-import]
  →  D3 device check  →  PR F5 release
```

| PR | Branch | What | New modules |
| --- | --- | --- | --- |
| F1 | `audio-package` | One package for reading audio; AIFF; convert at export; preview through Go; texts | none |
| F2 | `flac-import` | FLAC | `mewkiz/flac` |
| F3 | `mp3-ogg-import` | MP3 and Ogg Vorbis | `go-mp3`, `oggvorbis` |
| F4 | `akai-legacy-import` | `.snd`, `.s1s`, `.s3s`, if decision 9 says so | none |
| F5 | `release-<version>` | D3, documents, version | none |

Each PR starts from `main` after the one before it is merged. F1 carries all the plumbing with no new dependency, so the design can be judged before any module is added.

## PRs and tasks

### PR F1: `audio-package`

- [ ] New package `internal/audio`:
  - `Info`: format name, sample rate, channels, bits per sample, frame count.
  - `ReadFile(path) (Info, error)`: recognises the format from the first bytes (decision 7) and reads the header only.
  - `Validate`: mono or stereo, at most 20 minutes, and the format's own rules.
  - `Open(path)`: a reader that gives decoded frames in blocks.
  - `WriteWAV(w, path)`: writes the file as WAV per decisions 3 and 4, in blocks.
  - One place that recovers a decoder's panic and returns an error.
  - WAV is handled by calling `internal/wav`, which stays as it is.
- [ ] AIFF reader in `internal/audio`: `FORM`/`AIFF` and `FORM`/`AIFC`, the `COMM` and `SSND` chunks, the 80-bit sample rate, 8, 16, 24 and 32-bit integers, and the uncompressed AIFF-C kinds `NONE`, `sowt` and `fl32`. Refuse every other kind by name.
- [ ] `AssignSample` uses `audio.ReadFile` and `Validate`. For a non-WAV source the pool file gets a `.wav` name (decision 5). `sliceInfo.End` is the frame count minus 1, as now.
- [ ] Export: a WAV source is copied as now (decision 8); any other source goes through `audio.WriteWAV`. Folder and zip exports both. The link and missing-file checks stay in front of both paths. A file that fails to decode halfway fails the export with the file's name and leaves no half-written export, as a failed copy does today.
- [ ] Preview (`internal/app/audio.go`): for a non-WAV source, `/audio/<slot>` answers in the decoded form that `?pcm=1` gives (decision 6). The frontend picks the parser by the answer's `Content-Type`, not by trying the webview first.
- [ ] Accept the new extensions in: the file dialog's filter (one "Audio files" filter listing every extension supported so far), the file-drop handler, and the `-pad` flag's help.
- [ ] Texts: "Choose WAV file" becomes "Choose audio file", and the same for the other text in `PadPanel.svelte`, the grid hint and the "WAV files only" note in `App.svelte`, the dialog title in `dialogs.go` and the drop error in `main.go`. Error messages for an unsupported file name the formats that are supported.
- [ ] Tests:
  - AIFF files written in the test at each bit depth, mono and stereo, big and little-endian, decode to the same frames as the matching WAV (as `TestDecodeAgreesAcrossFormats` does for WAV).
  - Cut-off and damaged AIFF headers return errors, and a reader made to panic returns an error.
  - Assign an AIFF, export, load the export: the data folder holds a WAV with the same frames, and the pad's `End` matches.
  - A WAV export is byte-identical to before.
  - `TestAudio` covers the non-WAV answer.

**Done when** the tests pass, and on the laptop an AIFF made from the test tone can be chosen, dropped on a pad, previewed in full with the end beep, and exported as a WAV that plays the same in another program.

### PR F2: `flac-import`

- [ ] Add `github.com/mewkiz/flac`. Length, rate, channels and bit depth come from the stream's first block; refuse a file that leaves the total length out.
- [ ] Register the format in `internal/audio`, the dialog filter, the drop handler and the texts.
- [ ] Tests with the committed tone files: 16 and 24-bit, mono and stereo, decode to the same frames as the WAV they were made from. Cut-off files return errors.

**Done when** the tests pass, `govulncheck` is clean, and the laptop steps of F1 work with a FLAC.

### PR F3: `mp3-ogg-import`

- [ ] Add `github.com/hajimehoshi/go-mp3` and `github.com/jfreymuth/oggvorbis`.
- [ ] MP3: the length comes from the decoder's own count, which scans the file's frames without decoding them. Check that the count equals the frames `Open` then delivers; `sliceInfo.End` must match the WAV that export writes, to the frame.
- [ ] Ogg: Vorbis only. An Ogg file holding anything else is refused with a message that says so.
- [ ] Both write 16-bit WAVs (decision 4).
- [ ] Tests with the committed tone files: the decoded length is within 0.1 s of one second, the tone's pitch is right (count zero crossings), the exported WAV has exactly `End + 1` frames, and cut-off files return errors.
- [ ] Record in the PR's Notes how many frames each decoder gives for the tone files, next to the frame count the device used in D1.

**Done when** the tests pass, `govulncheck` is clean, and the laptop steps of F1 work with an MP3 and an Ogg file, including a long MP3 (several minutes) that exports without the app's memory growing by more than a few megabytes.

### PR F4: `akai-legacy-import` (only if decision 9 says so)

- [ ] From bajdev's files and the WAVs the device made from them in D1, work out each format's header: where the sample rate, channel count, length and audio data are. Write what was found into the format document before writing code.
- [ ] Readers in `internal/audio`, each refusing any file whose header does not match what was found.
- [ ] Tests: each supplied file decodes to the same frames as the device's WAV of it.

**Done when** every supplied file previews and exports like the device's own conversion. If a format cannot be worked out from the files at hand, leave it unsupported and say so in the PR record.

### PR F5: `release-<version>`

- [ ] Device check D3, recorded in a new `docs/checkpoint-d.md`.
- [ ] `docs/xpj-format-and-scope.md`: move "Other audio formats" out of "2.0 or later", change "Audio: WAV only", and add what D1 and D3 showed.
- [ ] `docs/build-plan.md`: the assign rule in section 7.
- [ ] README: the formats accepted, that non-WAV files are exported as WAV, and the size that means on the card.
- [ ] About: the modules added and their licences, where About or the README lists what MPCkit is built with.
- [ ] Version in every file the README's "Releasing" paragraph lists. After the merge, tag the merge commit.
- [ ] The MPCkit site (`mpckit-site`) gets its own PR for the new release.

**Done when** D3 is recorded with no open problem, the documents agree with the app, and the tag is on the merge commit.
