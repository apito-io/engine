# Use a minimal Alpine image for the final stage
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the binary from the build artifact location (this will be handled by GitHub Actions)
COPY engine /root/engine

# Expose port 5050
EXPOSE 5050

# Command to run the binary
CMD ["./engine"]
