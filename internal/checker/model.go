package checker

type ReportItem struct {
	Work1ID uint64 `json:"work1_id"`
	Work2ID uint64 `json:"work2_id"`

	Avg float64 `json:"avg"`
	Max float64 `json:"max"`

	Matches []MatchItem `json:"matches"`
}

type MatchItem struct {
	Work1File  string `json:"work1_file"`
	Work1Start uint64 `json:"work1_start"`
	Work1Size  uint64 `json:"work1_size"`

	Work2File  string `json:"work2_file"`
	Work2Start uint64 `json:"work2_start"`
	Work2Size  uint64 `json:"work2_size"`
}

type ResultDTO struct {
	ID1              string        `json:"id1"`
	ID2              string        `json:"id2"`
	Similarities     SimilarityDTO `json:"similarities"`
	Matches          []MatchDTO    `json:"matches"`
	FirstSimilarity  float64       `json:"first_similarity"`
	SecondSimilarity float64       `json:"second_similarity"`
}

type SimilarityDTO struct {
	Avg float64 `json:"AVG"`
	Max float64 `json:"MAX"`
}

type MatchDTO struct {
	File1 string `json:"file1"`
	File2 string `json:"file2"`

	Start1 uint64 `json:"start1"`
	Start2 uint64 `json:"start2"`

	Start1Col uint64 `json:"start1_col"`
	Start2Col uint64 `json:"start2_col"`

	End1 uint64 `json:"end1"`
	End2 uint64 `json:"end2"`

	End1Col uint64 `json:"end1_col"`
	End2Col uint64 `json:"end2_col"`
}
