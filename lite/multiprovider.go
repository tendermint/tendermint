package lite

// multiProvider allows you to place one or more caches in front of a source
// Provider.  It runs through them in order until a match is found.
type multiProvider struct {
	Providers []Provider
}

// NewMultiProvider returns a new provider which wraps multiple other providers.
func NewMultiProvider(providers ...Provider) multiProvider {
	return multiProvider{
		Providers: providers,
	}
}

// SaveFullCommit saves on all providers, and aborts on the first error.
func (mc multiProvider) SaveFullCommit(fc FullCommit) (err error) {
	for _, p := range mc.Providers {
		err = p.SaveFullCommit(fc)
		if err != nil {
			return
		}
	}
	return
}

// LatestFullCommit loads the latest from all providers and provides
// the latest FullCommit that satisfies the conditions.
// Returns the first error encountered.
func (mc multiProvider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (fc FullCommit, err error) {
	for _, p := range mc.Providers {
		var fc_ FullCommit
		fc_, err = p.LatestFullCommit(chainID, minHeight, maxHeight)
		if err != nil {
			return
		}
		if fc_.Height() > fc.Height() {
			fc = fc_
		}
		if fc.Height() == maxHeight {
			return
		}
	}
	return
}
