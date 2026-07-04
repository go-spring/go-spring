# /gs Hello, World!

In day-to-day Go-Spring work, `gs` is a command-line tool. Think of it as a small crew that helps you get things done around a project: `gs init` bootstraps a new one, `gs-http-gen` turns an IDL into HTTP server and client code, `gs-mock` scaffolds a bit of test plumbing. If all you need is a service that follows Go-Spring's conventions, these commands are usually enough.

Framed narrowly, `gs` solves the "cold start" problem — how to lay out the directory, generate the code, and cut down on boilerplate. Once the project is up, it quietly steps back and the team takes over.

Now the AI era is here, and `gs` needs an upgrade to keep up. After the upgrade, it becomes `/gs`.

The extra slash is the call sigil inside an AI conversation. `/gs` is not a new CLI — it's an AI Skill built around Go-Spring projects. It calls the underlying `gs` tools, but it also reads project context, understands Go-Spring conventions, runs builds and tests, writes docs, and organizes changes. A rough way to tell them apart: `gs` is the hammer, `/gs` is the person who knows when to swing it and cleans up afterwards.

From here, let's look at how far that shift can carry the development process.

## From "starting a project" to "living with a project"

A traditional scaffold stands at day zero of a project. It lays out the directories, installs the dependencies, and gets an example running — then its job is done. Whatever happens after that isn't really its concern.

The problem is, real engineering is never a one-shot thing. Once a Go-Spring project is running, it still has to survive changing requirements, new endpoints, config tweaks, component onboarding, missing tests, broken builds, production incidents, and doc drift. Most of a team's time is spent in this "afterwards." What the project eventually looks like depends on whether each change respects the same engineering standard — not on how pretty the initial skeleton was.

What `/gs` wants to take on is precisely that "afterwards":

- **Requirements** — take a vague natural-language description and break it into concrete engineering tasks
- **Design** — reason about module boundaries, dependencies, and how Go-Spring components should be arranged
- **Coding** — make changes that follow the project's existing style, instead of dropping in an isolated snippet
- **Testing** — fill in the unit tests, integration tests, or examples along the way
- **Building** — know which module to enter so `go test` and `go build` land in the right place
- **Delivery** — put together the change summary, the risks, and the verification results
- **Retention** — write the lessons learned and conventions back into docs and Skills

When the same entry point strings all of this together, `/gs` stops being merely a "help me write some code" assistant. It becomes more like "help me get this done the Go-Spring way."

## A middleware sitting inside the development flow

Anyone familiar with Go-Spring knows the word "middleware": the framework lets you hang cross-cutting concerns — logging, auth, tracing — onto the request path or the application lifecycle. `/gs` does something similar, only inside the development flow.

It doesn't make decisions for you, but it steps in at a few key moments to make sure the standard motions actually happen:

- When a requirement arrives, it presses for clarity — goals, boundaries, and acceptance criteria
- When a design takes shape, it pulls any implementation that drifts from the project structure back into the Go-Spring component model
- When code is being changed, it holds the line on existing directory layout, naming, error handling, and configuration conventions
- When tests run, it turns "changed and shipped" into "changed and verified"
- When a build fails, it doesn't just hand back the red text — it points at where the problem is and suggests a way through
- When something is being submitted, it fills in the change summary, verification status, and remaining risk

The point of all this: `/gs` isn't a smarter code generator. It's more like adding a few checkpoints to the flow so that conventions move from "written in the docs" to "walked through every time."

## What does "engineering discipline" mean in the AI era

When people talk about the AI era, the first instinct is often to let the model generate freely. But for a team doing real engineering, the more valuable direction is the opposite — draw a clear boundary and let the model work inside it.

For Go-Spring projects, the boundary is mostly common sense:

- The repository root is not necessarily a Go module — builds have to go into the specific subproject
- Configuration, logging, IoC, and lifecycle are managed by the framework
- Prefer an existing Starter when integrating a component
- Errors need enough context when they're propagated upward
- Dependency injection happens at startup; no runtime dynamic injection
- Beans of the same name and type should not rely on implicit overrides — pick one explicitly through mutually exclusive conditions
- Tests, examples, and docs evolve with the code, not as an afterthought

If these conventions live only in people's heads, they warp as the project grows. What `/gs` does is translate them into context an AI can read and act on.

A few everyday scenarios make it plainer:

- Someone says "add an endpoint" — the right response isn't just spitting out a handler. It's asking which module it belongs to, whether the IDL needs to change, whether new config is required, how dependencies are injected, how errors are wrapped, how tests should cover it, and whether docs need an update.
- Someone says "just build it" — you can't assume there's a `go.mod` at the root. Go-Spring repos are usually stitched together from multiple modules, so the right directory has to be picked first.
- Someone says "wire up Redis" — no need to write initialization from scratch. Check whether a Starter already exists; if so, reuse it and let Go-Spring handle the config binding, bean registration, and lifecycle.

That's roughly what engineering discipline looks like in the AI era: context-driven, convention-executed, and every change converges to a verifiable result.

## How `/gs` gets grounded

Roughly three layers.

### 1. Make the project context explicit

For an AI to participate reliably, the first thing it needs is to actually understand the project.

A Go-Spring project can put the key information in plain sight through `AGENTS.md`, coding style guides, directory conventions, sample projects, and documentation:

- How the project is organized
- Which directories are independent Go modules
- The common build, test, and run commands
- What is expected around error handling and logging
- Which examples to imitate when adding a new feature
- Which changes require a doc update

The clearer this is, the more consistently `/gs` behaves. The team doesn't have to re-explain the rules every time, and the AI doesn't have to guess.

### 2. Turn common actions into flows

The second step is to lock down high-frequency actions as flows.

"Adding an HTTP endpoint" sounds like writing a few lines of code, but unpacked it's a sequence with a clear order:

1. Pin down the semantics, inputs, and outputs
2. Decide whether the IDL needs a change and code needs to be regenerated
3. Find the target module and its existing layering
4. Modify the handler, service, config, and registration
5. Add tests or examples
6. Run gofmt, tests, and the build
7. Summarize the changes and verification

`/gs` can run this sequence by default. The user states the goal, the AI works through the middle, and where a real choice has to be made, it explains what was picked and why.

### 3. Push capability into tools

The third step is for `/gs` to lean on tools instead of stuffing logic into prompts.

A Go-Spring project already has a decent toolkit sitting right there:

- `gs init` — scaffold a project that respects conventions
- `gs-http-gen` — generate HTTP code from an IDL
- `gs-mock` — generate test helper code
- `go test` — verify module behavior
- `go build` — verify the artifact builds
- `gofmt` — keep formatting consistent
- Documentation build tools — confirm the site still renders

The value of an AI Skill is knowing which tool to invoke when, and being able to read what the tool's output actually means for the project. It isn't a natural-language wrapper on top of a CLI — it's a CLI plugged into a full development flow.

## What a `/gs` request looks like end to end

In the ideal case, the user's input can be as short as this:

```text
/gs Add a "get user detail" HTTP endpoint to the user service, keyed by user_id, and update the tests and docs while you're at it.
```

What `/gs` needs to do goes well beyond "write an endpoint":

- Read the project structure and locate the module for the user service
- Figure out how the existing IDL, routing, handler, service, and repository are organized
- Generate or modify code so naming and layering match the current style
- Use Go-Spring's IoC, config, and lifecycle to organize dependencies
- Wrap errors the way this project does, keeping enough context to trace them
- Add tests covering the happy path and a few key failure paths
- Run gofmt, go test, and go build when needed
- Update the docs or examples to describe the endpoint's behavior
- Wrap up with a change summary, verification commands, and remaining risks

Underneath, this is a different way of working: instead of switching tools by hand one at a time, you hand the goal to `/gs` and let it break the goal into a series of standard motions.

## What this means for Go-Spring

Go-Spring has always been about engineering discipline on the application side — configuration, dependency injection, logging, lifecycle, component integration. All of it together is meant to keep Go services simple while giving them a steadier foundation.

`/gs` pushes that discipline one step earlier, into the development process itself.

The framework handles how the application organizes itself at runtime; `/gs` handles how development organizes itself. One makes startup, wiring, and shutdown more predictable; the other makes requirements, code, tests, and delivery more predictable. Put them together, and Go-Spring's engineering discipline lives not only in the code but in how the team works day to day:

- A unified lifecycle at runtime
- A single entry point during development
- A standard way to onboard components
- A reusable flow for turning requirements into code
- Default verification steps for every change
- A stable place for hard-won lessons to accumulate

## After "Hello, World"

"Hello, World" is usually a technology's first greeting to the outside world.

`/gs Hello, World!` isn't a minimal example — it's a new entry point. From here on, requirements in a Go-Spring project can be understood, code can be changed, tests can be run, builds can be verified, docs can be kept up to date, and lessons can actually stick.

`gs` makes starting a project easier. `/gs` makes living with one easier.

What matters in the AI era isn't the occasional working snippet an AI produces — it's whether AI can consistently participate in a development flow that is reusable, verifiable, and worth keeping around. For Go-Spring, `/gs` is the entry point to that flow.
