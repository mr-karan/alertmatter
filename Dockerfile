FROM ubuntu:22.04

# Update and install necessary packages
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user and switch to it
RUN useradd -m alertmatter
USER alertmatter

# Set working directory
WORKDIR /app

# Copy the binary
COPY alertmatter.bin .

# Set the entrypoint
EXPOSE 7777
ENTRYPOINT ["./alertmatter.bin"]
CMD ["--addr=0.0.0.0:7777", "--webhook-url=https://mattermost.internal/webhook"]
