package persist

type persistStorage struct {
	filePath   string
	restore    bool
	storeInter int
}

func NewPersistStorage(filePath string, restore bool, storeInter int) *persistStorage {
	return &persistStorage{
		filePath:   filePath,
		restore:    restore,
		storeInter: storeInter,
	}
}

// func NewProducer(fileName string) (*Producer, error) {
// 	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &Producer{
// 		file:    file,
// 		encoder: json.NewEncoder(file),
// 	}, nil
// }

// func (p *Producer) WriteEvent(event *Event) error {
// 	return p.encoder.Encode(&event)
// }

// func NewConsumer(fileName string) (*Consumer, error) {
// 	file, err := os.OpenFile(fileName, os.O_RDONLY|os.O_CREATE, 0666)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &Consumer{
// 		file:    file,
// 		decoder: json.NewDecoder(file),
// 	}, nil
// }

// func (c *Consumer) ReadEvent() (*Event, error) {
// 	event := &Event{}
// 	if err := c.decoder.Decode(&event); err != nil {
// 		return nil, err
// 	}

// 	return event, nil
// }
