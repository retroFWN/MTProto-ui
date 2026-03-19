#!/bin/bash

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}"
echo "  __  __ _____ ____                        ____                  _ "
echo " |  \/  |_   _|  _ \ _ __ _____  ___   _  |  _ \ __ _ _ __   ___| |"
echo " | |\/| | | | | |_) | '__/ _ \ \/ / | | | | |_) / _\` | '_ \ / _ \ |"
echo " | |  | | | | |  __/| | | (_) >  <| |_| | |  __/ (_| | | | |  __/ |"
echo " |_|  |_| |_| |_|   |_|  \___/_/\_\\\\__, | |_|   \__,_|_| |_|\___|_|"
echo "                                    |___/                            "
echo -e "${NC}"
echo "MTProxy Panel Installer v1.0.0 (Go Edition)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo)"
    exit 1
fi

# Install Docker if not present
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}Installing Docker...${NC}"
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    echo -e "${GREEN}Docker installed.${NC}"
fi

# Install Docker Compose if not present
if ! docker compose version &> /dev/null 2>&1; then
    echo -e "${YELLOW}Installing Docker Compose plugin...${NC}"
    apt-get install -y docker-compose-plugin 2>/dev/null || true
fi

INSTALL_DIR="/opt/mtproxy-panel"
echo -e "${YELLOW}Installing to ${INSTALL_DIR}...${NC}"

# Copy project files
mkdir -p "$INSTALL_DIR"
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cp -r "$SCRIPT_DIR"/* "$INSTALL_DIR/" 2>/dev/null
cp -r "$SCRIPT_DIR"/.* "$INSTALL_DIR/" 2>/dev/null || true

cd "$INSTALL_DIR"

# Pull proxy image
echo -e "${YELLOW}Pulling MTProto proxy image...${NC}"
docker pull telegrammessenger/proxy

# Build and start via Docker Compose
echo -e "${YELLOW}Building and starting panel...${NC}"
docker compose up -d --build

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
echo -e "  ${YELLOW}Change the default password after first login!${NC}"
echo ""
echo "  Commands:"
echo "    cd $INSTALL_DIR && docker compose logs -f    - view logs"
echo "    cd $INSTALL_DIR && docker compose restart    - restart"
echo "    cd $INSTALL_DIR && docker compose down       - stop"
echo ""
