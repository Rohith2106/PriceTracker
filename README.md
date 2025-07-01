# 🚀 Price Tracker 🚀

Welcome to **Price Tracker**, a powerful and sleek web application designed to help you monitor product prices from your favorite online stores. Never miss a deal again!

## 📸Screenshot
![img1](https://raw.githubusercontent.com/Rohith2106/PriceTracker/main/images/img1.png)
![img2](https://raw.githubusercontent.com/Rohith2106/PriceTracker/main/images/img2.png)
![img3](https://raw.githubusercontent.com/Rohith2106/PriceTracker/main/images/img3.png)


---

## ✨ Features

- **Real-time Price Checking**: Instantly check the current price of any product.
- **Price Tracking**: Monitor products and get notified when the price drops below your target.
- **Web Push Notifications**: Receive instant alerts in your browser, even when the app is not open.
- **Dynamic Scraping**: Intelligently finds prices on various e-commerce sites.
- **Modern UI**: A clean and intuitive user interface built with the latest web technologies.

---

## 🛠️ Tech Stack

This project is a full-stack application built with a Go backend and a React/Next.js frontend.

### 📦 Backend (Go)

The backend is responsible for the core logic, including web scraping, price tracking, and sending notifications.

| Functionality         | Go Packages Used                                                                                             |
| --------------------- | ------------------------------------------------------------------------------------------------------------ |
| **Web Scraping**      | `gocolly/colly` - A robust and fast scraping framework.                                                      |
| **HTTP Server**       | `gorilla/mux` - A powerful URL router and dispatcher.                                                        |
| **Real-time Comms**   | `gorilla/websocket` - For real-time communication with the frontend.                                         |
| **CORS Handling**     | `rs/cors` - For handling Cross-Origin Resource Sharing.                                                      |
| **Web Push Notifs**   | `SherClockHolmes/webpush-go` - For sending VAPID-secured web push notifications.                             |

### 🎨 Frontend (React & Next.js)

The frontend provides a seamless and interactive user experience for tracking products.

| Functionality         | Key Libraries/Frameworks                                                                                     |
| --------------------- | ------------------------------------------------------------------------------------------------------------ |
| **Framework**         | `next` - A leading React framework for building server-rendered and static web applications.                 |
| **UI Library**        | `react` - For building dynamic and component-based user interfaces.                                          |
| **Styling**           | `tailwindcss` - A utility-first CSS framework for rapid UI development.                                      |
| **Linting**           | `eslint` - To maintain code quality and consistency.                                                         |

---

## 🚀 Getting Started

Follow these instructions to get the project up and running on your local machine.

### ✅ Prerequisites

- [Go](https://go.dev/doc/install) (v1.24 or later)
- [Node.js](https://nodejs.org/en/download/) (v18 or later)
- `npm` or `yarn` package manager

### ⚙️ Installation & Running

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/PriceTracker.git
    cd PriceTracker
    ```

2.  **Run the Backend Server:**
    Navigate to the `backend` directory and start the Go server.
    ```bash
    cd backend
    go mod tidy # Installs dependencies
    go run main.go
    ```
    The backend will be running on `http://localhost:8080`.

3.  **Run the Frontend Application:**
    In a new terminal, navigate to the `frontend` directory and start the development server.
    ```bash
    cd ../frontend
    npm install
    npm run dev
    ```
    The frontend will be accessible at `http://localhost:3000`.

---

## 📂 Project Structure

The repository is organized into two main directories:

```
PriceTracker/
├── backend/         # Go Backend Source Code
│   ├── main.go      # Main application entry point
│   ├── go.mod       # Go module dependencies
│   ├── scraper/     # Web scraping logic
│   └── tracker/     # Price tracking and notification logic
└── frontend/        # Next.js Frontend Source Code
    ├── app/         # Core Next.js app directory
    ├── public/      # Static assets
    ├── package.json # NPM dependencies
    └── next.config.mjs # Next.js configuration
```

