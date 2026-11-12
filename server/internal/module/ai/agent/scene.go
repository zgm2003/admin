package agent

type Scene int16

const (
	SceneConversation    Scene = 1
	SceneToolGeneration  Scene = 2
	SceneImageGeneration Scene = 3
	SceneVideoGeneration Scene = 4
	SceneAudioGeneration Scene = 5
)

func (scene Scene) Valid() bool {
	return scene >= SceneConversation && scene <= SceneAudioGeneration
}
