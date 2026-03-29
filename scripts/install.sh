#!/bin/bash

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}"
echo "  __  __ _____ ____                        ____                  _ "
echo " |  \/  |_   _|  _ \ _ __ _____  ___   _  |  _ \ __ _ _ __   ___| |"
echo " | |\/| | | | | |_) | '__/ _ \ \/ / | | | | |_) / _\` | '_ \ / _ \ |"
echo " | |  | | | | |  __/| | | (_) >  <| |_| | |  __/ (_| | | | |  __/ |"
echo " |_|  |_| |_| |_|   |_|  \___/_/\_\\\\__, | |_|   \__,_|_| |_|\___|_|"
echo "                                    |___/                            "
echo -e "${NC}"
echo "MTProxy Panel Installer v2.0.0 (Go Edition)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root (sudo)${NC}"
    exit 1
fi

# Install Docker if not present
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}Installing Docker...${NC}"
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker
    echo -e "${GREEN}Docker installed.${NC}"
fi

# Install Docker Compose if not present
if ! docker compose version &> /dev/null 2>&1; then
    echo -e "${YELLOW}Installing Docker Compose plugin...${NC}"
    apt-get install -y docker-compose-plugin 2>/dev/null || true
fi

INSTALL_DIR="/opt/MTProto-ui"
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Copy project files to install directory
mkdir -p "$INSTALL_DIR"
echo -e "${YELLOW}Copying files to ${INSTALL_DIR}...${NC}"
cp -r "$SCRIPT_DIR"/* "$INSTALL_DIR/" 2>/dev/null
cp -r "$SCRIPT_DIR"/.* "$INSTALL_DIR/" 2>/dev/null || true

cd "$INSTALL_DIR" || { echo -e "${RED}Failed to cd to ${INSTALL_DIR}${NC}"; exit 1; }

# Stop old panel container if exists (e.g. from previous install location)
if docker ps -a --format '{{.Names}}' | grep -q '^mtproxy-panel$'; then
    echo -e "${YELLOW}Stopping old mtproxy-panel container...${NC}"
    docker stop mtproxy-panel 2>/dev/null
    docker rm mtproxy-panel 2>/dev/null
fi

# Pre-build telemt (Rust) proxy image
echo -e "${YELLOW}Building telemt (Rust) proxy image...${NC}"
if ! docker images -q telemt-local 2>/dev/null | grep -q .; then
    docker build -t telemt-local https://github.com/telemt/telemt.git
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}telemt image built successfully.${NC}"
    else
        echo -e "${RED}Failed to build telemt image. Rust backend will not be available.${NC}"
    fi
else
    echo -e "${GREEN}telemt image already exists, skipping build.${NC}"
fi

# Build and start via Docker Compose
echo -e "${YELLOW}Building and starting panel...${NC}"
docker compose up -d --build --wait
if [ $? -ne 0 ]; then
    echo -e "${RED}Docker Compose failed! Check logs: docker compose logs${NC}"
    exit 1
fi

SERVER_IP=$(curl -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  MTProxy Panel installed successfully!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  Panel URL:  ${BLUE}http://${SERVER_IP}:8080${NC}"
echo -e "  Username:   ${YELLOW}admin${NC}"
echo -e "  Password:   ${YELLOW}admin${NC}"
echo ""
echo -e "  ${RED}Смените пароль после первого входа!${NC}"
echo ""
echo "  Environment variables (docker-compose.yml):"
echo "    PANEL_PORT=8080         - Panel port"
echo "    PANEL_DOMAIN=           - Domain for auto-SSL"
echo "    PROXY_BACKEND=official  - Engine: official (C) or telemt (Rust)"
echo ""
echo "  Commands:"
echo "    cd $INSTALL_DIR && docker compose logs -f    - view logs"
echo "    cd $INSTALL_DIR && docker compose restart    - restart"
echo "    cd $INSTALL_DIR && docker compose down       - stop"
echo ""
echo "  Update:"
echo "    cd $INSTALL_DIR && git pull && docker compose up -d --build"
echo ""
