package services

import (
	"context"
	"fmt"
	"github.com/apito-io/engine/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

const (
	CharSet = "UTF-8" // Character Set
)

func SendEmail(ctx context.Context, _awsConfig aws.Config, req *models.EmailSendRequest) error {

	// Create SES client
	svc := ses.NewFromConfig(_awsConfig)

	// Define email message
	input := &ses.SendEmailInput{
		Source: aws.String(req.Sender),
		Destination: &types.Destination{
			ToAddresses: req.Recipients,
		},
		Message: &types.Message{
			Subject: &types.Content{
				Charset: aws.String(CharSet),
				Data:    aws.String(req.Subject),
			},
			Body: &types.Body{
				Html: &types.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(req.HtmlBody),
				},
				Text: &types.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(req.TextBody),
				},
			},
		},
	}

	// Send the email
	result, err := svc.SendEmail(ctx, input)
	if err != nil {
		return err
	}

	fmt.Printf("Email sent successfully, Message ID: %s\n", *result.MessageId)

	return nil
}

func SendTeamAddEmail(ctx context.Context, _awsConfig aws.Config, req *models.EmailSendRequest) error {
	req.Subject = fmt.Sprintf("Welcome to Apito.io! You've been added to a new project")
	if req.TempPassword != "" {
		req.TextBody = fmt.Sprintf(`
	Hi,

	Welcome to the %s project! You have been added to the project. Your temporary password is: %s.

	Please log in and change your password as soon as possible.

	This is an automated email, please do not reply. If you need assistance, contact your administrator.

	Best regards,
	Apito.io Team
	`, req.ProjectName, req.TempPassword)
		req.HtmlBody = fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Welcome to Apito.io</title>
		<style>
			body {
				font-family: Arial, sans-serif;
				background-color: #f4f4f4;
				margin: 0;
				padding: 0;
			}
			.email-container {
				max-width: 600px;
				margin: 0 auto;
				background-color: #ffffff;
				border: 1px solid #dddddd;
				border-top: 10px solid #EA3A60; /* Top 10px border with the brand color */
			}
			.header {
				text-align: center;
				padding: 40px 0;
			}
			.header img {
				max-width: 100px;
			}
			.header-title {
				font-size: 28px;
				color: #000000;
				margin: 20px 0 10px;
			}
			.body {
				padding: 20px;
				text-align: center;
			}
			.body p {
				font-size: 16px;
				color: #333333;
				line-height: 1.5;
			}
			.body a {
				display: inline-block;
				background-color: #007bff;
				color: white;
				text-decoration: none;
				padding: 15px 30px;
				font-size: 16px;
				border-radius: 5px;
				margin: 20px 0;
			}
			.footer {
				background-color: #f4f4f4;
				padding: 10px;
				text-align: center;
				font-size: 12px;
				color: #888888;
			}
			.footer a {
				color: #EA3A60;
				text-decoration: none;
			}
			.warning {
				color: #EA3A60;
				font-weight: bold;
			}
		</style>
	</head>
	<body>
		<div class="email-container">
			<!-- Header Section -->
			<div class="header">
				<img width="50" height="50" src="https://apito.io/img/logo.png" alt="Apito.io Logo">
				<h1 class="header-title">Welcome to %s</h1>
			</div>
	
			<!-- Body Section -->
			<div class="body">
				<p>Hi,</p>
				<p>You have been added to the project, and here is your temporary login password:</p>
				<p><strong>Password: %s </strong></p>
				<p>Please log in and change your password as soon as possible.</p>
				<p class="warning">This is an automated email, please do not reply. If you need assistance, contact your administrator.</p>
			</div>
	
			<!-- Footer Section -->
			<div class="footer">
				<p>Best regards,<br>Apito.io Team</p>
				<p>Visit us at <a href="https://apito.io">Apito.io</a></p>
			</div>
		</div>
	</body>
	</html>
	`, req.ProjectName, req.TempPassword)
	} else {
		req.TextBody = fmt.Sprintf(`
	Hi,

	Welcome to the %s project! You have been added to the project.

	This is an automated email, please do not reply. If you need assistance, contact your administrator.

	Best regards,
	Apito.io Team
	`, req.ProjectName)
		req.HtmlBody = fmt.Sprintf(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Welcome to Apito.io</title>
		<style>
			body {
				font-family: Arial, sans-serif;
				background-color: #f4f4f4;
				margin: 0;
				padding: 0;
			}
			.email-container {
				max-width: 600px;
				margin: 0 auto;
				background-color: #ffffff;
				border: 1px solid #dddddd;
				border-top: 10px solid #EA3A60; /* Top 10px border with the brand color */
			}
			.header {
				text-align: center;
				padding: 40px 0;
			}
			.header img {
				max-width: 100px;
			}
			.header-title {
				font-size: 28px;
				color: #000000;
				margin: 20px 0 10px;
			}
			.body {
				padding: 20px;
				text-align: center;
			}
			.body p {
				font-size: 16px;
				color: #333333;
				line-height: 1.5;
			}
			.body a {
				display: inline-block;
				background-color: #007bff;
				color: white;
				text-decoration: none;
				padding: 15px 30px;
				font-size: 16px;
				border-radius: 5px;
				margin: 20px 0;
			}
			.footer {
				background-color: #f4f4f4;
				padding: 10px;
				text-align: center;
				font-size: 12px;
				color: #888888;
			}
			.footer a {
				color: #EA3A60;
				text-decoration: none;
			}
			.warning {
				color: #EA3A60;
				font-weight: bold;
			}
		</style>
	</head>
	<body>
		<div class="email-container">
			<!-- Header Section -->
			<div class="header">
				<img width="50" height="50" src="https://apito.io/img/logo.png" alt="Apito.io Logo">
				<h1 class="header-title">Welcome to %s</h1>
			</div>
	
			<!-- Body Section -->
			<div class="body">
				<p>Hi,</p>
				<p>You have been added to the project.</p>
				<p>Please log in with your existing email and password.</p>
				<p class="warning">This is an automated email, please do not reply. If you need assistance, contact your administrator.</p>
			</div>
	
			<!-- Footer Section -->
			<div class="footer">
				<p>Best regards,<br>Apito.io Team</p>
				<p>Visit us at <a href="https://apito.io">Apito.io</a></p>
			</div>
		</div>
	</body>
	</html>
	`, req.ProjectName)
	}
	return SendEmail(ctx, _awsConfig, req)
}
