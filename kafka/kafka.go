package kafka

import (
	"net/smtp"
	"os"

	kafka "github.com/IBM/sarama"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Kafka struct {
	Logger   *zap.Logger
	Producer kafka.SyncProducer
	Consumer kafka.Consumer
}

func New(logger *zap.Logger) *Kafka {
	config := kafka.NewConfig()
	config.Producer.RequiredAcks = kafka.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = 5

	producer, err := kafka.NewSyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		logger.Fatal("Failed to create sync producer: ", zap.Error(err))
	}
	consumer, err := kafka.NewConsumer([]string{"localhost:9092"}, config)
	if err != nil {
		logger.Fatal("Failed to create consumer: ", zap.Error(err))
	}
	defer consumer.Close()
	defer producer.Close()

	return &Kafka{
		Logger:   logger,
		Producer: producer,
		Consumer: consumer,
	}
}

func (k *Kafka) SendMessage(message string) error {
	go func() {
		msg := &kafka.ProducerMessage{
			Topic: "test-topic",
			Key:   nil,
			Value: kafka.StringEncoder(message),
		}

		_, _, err := k.Producer.SendMessage(msg)
		if err != nil {
			k.Logger.Error("Failed to send message", zap.Error(err))
		}
	}()

	k.Logger.Info("Sent message", zap.String("message", message))

	return nil
}

func (k *Kafka) WaitForMessage(email string) error {
	err := godotenv.Load()
	if err != nil {
		k.Logger.Error("Error loading .env file")
	}

	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	to := []string{email}

	subject := "Subject: Registration on our shop\n"

	from := os.Getenv("EMAIL_ADDRESS")
	password := os.Getenv("EMAIL_PASSWORD")

	auth := smtp.PlainAuth("", from, password, smtpHost)

	partitionConsumer, err := k.Consumer.ConsumePartition("test-topic", 0, kafka.OffsetNewest)
	if err != nil {
		k.Logger.Error("Failed to create partition consumer: ", zap.Error(err))
	}
	defer partitionConsumer.Close()

	go func() {
		for msg := range partitionConsumer.Messages() {
			message := []byte(subject + "\n" + string(msg.Value))

			err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, message)
			if err != nil {
				k.Logger.Error("Failed to send email", zap.Error(err))
			}
		}
	}()

	return nil
}
