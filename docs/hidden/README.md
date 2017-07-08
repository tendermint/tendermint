# Architecture Decision Records

This is a location to record all high-level architecture decisions in the tendermint project.  Not the implementation details, but the reasoning that happened.  This should be refered to for guidance of the "right way" to extend the application.  And if we notice that the original decisions were lacking, we should have another open discussion, record the new decisions here, and then modify the code to match.

This is like our guide and mentor when Jae and Bucky are offline.... The concept comes from a [blog post](https://product.reverb.com/documenting-architecture-decisions-the-reverb-way-a3563bb24bd0#.78xhdix6t) that resonated among the team when Anton shared it.

Each section of the code can have it's own markdown file in this directory, and please add a link to the readme.

## Sections

* [ABCI](./ABCI.md)
* [go-merkle / merkleeyes](./merkle.md)
* [Frey's thoughts on the data store](./merkle-frey.md)
* basecoin
* tendermint core (multiple sections)
* ???
