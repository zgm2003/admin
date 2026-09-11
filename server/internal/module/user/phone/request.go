package phone

type sendCodeRequest struct {
	Target      *string `json:"target" binding:"required"`
	Phone       *string `json:"phone"`
	ChallengeID *string `json:"challengeId" binding:"omitempty,max=128"`
}

type bindOrChangeRequest struct {
	CurrentChallengeID *string `json:"currentChallengeId" binding:"omitempty,max=128"`
	CurrentCode        *string `json:"currentCode" binding:"omitempty,len=6,numeric"`
	NextPhone          *string `json:"nextPhone" binding:"required"`
	NextChallengeID    *string `json:"nextChallengeId" binding:"required,max=128"`
	NextCode           *string `json:"nextCode" binding:"required,len=6,numeric"`
}
