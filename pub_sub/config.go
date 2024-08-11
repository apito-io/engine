package pub_sub

type AMQPConfig struct {
	Exchange     string
	ExchangeType string
}

type RabbitMQConfig struct {
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQHost     string
	RabbitMQPort     string
	AMQP             AMQPConfig

	Environment string
}
