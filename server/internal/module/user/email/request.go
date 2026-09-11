package email

type sendCodeRequest struct {
	Target      *string `json:"target"`
	Email       *string `json:"email"`
	ChallengeID *string `json:"challengeId"`
}

type bindOrChangeRequest struct {
	CurrentChallengeID *string `json:"currentChallengeId"`
	CurrentCode        *string `json:"currentCode"`
	NextEmail          *string `json:"nextEmail"`
	NextChallengeID    *string `json:"nextChallengeId"`
	NextCode           *string `json:"nextCode"`
}
