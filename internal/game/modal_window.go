package game

// ModalButton is a button in a modal window.
type ModalButton struct {
	ID   uint8
	Text string
}

// ModalChoice is a choice in a modal window.
type ModalChoice struct {
	ID   uint8
	Text string
}

// ModalWindow is a modal dialog window sent to the client.
type ModalWindow struct {
	ID                  uint32
	Title               string
	Message             string
	Buttons             []ModalButton
	Choices             []ModalChoice
	DefaultEscapeButton uint8
	DefaultEnterButton  uint8
	Priority            bool
}
