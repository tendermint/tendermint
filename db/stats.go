package db

func mergeStats(src, dest map[string]string, prefix string) {
	for key, value := range src {
		dest[prefix+key] = value
	}
}
