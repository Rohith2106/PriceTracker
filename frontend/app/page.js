"use client";

import { useState, useEffect, useRef } from 'react';

export default function Home() {
  const [productUrl, setProductUrl] = useState('');
  const [targetPrice, setTargetPrice] = useState('');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [notificationPermission, setNotificationPermission] = useState('default');
  const [monitoring, setMonitoring] = useState(false);
  const [monitoredItems, setMonitoredItems] = useState([]);
  const [wsConnected, setWsConnected] = useState(false);
  const [sentNotifications, setSentNotifications] = useState(new Set());
  const wsRef = useRef(null);

  // URL slicing function
  const sliceProductUrl = (url) => {
    try {
      const urlObj = new URL(url);
      if (urlObj.hostname.includes('amazon')) {
        const pathParts = urlObj.pathname.split('/').filter(part => part);
        if (pathParts.length > 0) {
          return pathParts[0].replace(/-/g, ' ');
        }
      }
      return 'Product';
    } catch {
      return 'Product';
    }
  };

  // Request notification permission and setup WebSocket connection
  useEffect(() => {
    // Only run on client side
    if (typeof window === 'undefined') return;

    // Request notification permission
    if ('Notification' in window) {
      if (Notification.permission === 'default') {
        Notification.requestPermission().then(permission => {
          setNotificationPermission(permission);
        });
      } else {
        setNotificationPermission(Notification.permission);
      }
    }

    // Setup WebSocket connection
    const connectWebSocket = () => {
      console.log('Attempting to connect to WebSocket...');
      const ws = new WebSocket('ws://localhost:8080/ws');
      wsRef.current = ws;

      ws.onopen = () => {
        console.log('WebSocket connected successfully');
        setWsConnected(true);
      };

      ws.onmessage = (event) => {
        console.log('Raw WebSocket message received:', event.data);
        try {
          const alert = JSON.parse(event.data);
          console.log('Parsed price alert:', alert);
          
          // Check if notification already sent for this item
          if (!sentNotifications.has(alert.ID)) {
            // Show OS notification
            if (notificationPermission === 'granted') {
              console.log('Showing OS notification...');
              new Notification('Price Alert!', {
                body: `${sliceProductUrl(alert.URL)} price dropped to ₹${alert.currentPrice}! Target was ₹${alert.targetPrice}`,
                icon: '/favicon.ico'
              });
            } else {
              console.log('Notification permission not granted:', notificationPermission);
            }
            
            // Update UI message
            setMessage(`🎉 Price Alert! ${sliceProductUrl(alert.URL)} dropped to ₹${alert.currentPrice}!`);
            
            // Mark notification as sent
            setSentNotifications(prev => new Set([...prev, alert.ID]));
            
            // Stop monitoring this item
            stopMonitoring(alert.ID);
          }
        } catch (error) {
          console.error('Error parsing WebSocket message:', error);
        }
      };

      ws.onclose = (event) => {
        console.log('WebSocket disconnected:', event.code, event.reason);
        setWsConnected(false);
        // Reconnect after 3 seconds
        setTimeout(connectWebSocket, 3000);
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        setWsConnected(false);
      };
    };

    connectWebSocket();

    // Cleanup on unmount
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [notificationPermission, sentNotifications]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setMessage('');

    try {
      const response = await fetch('http://localhost:8080/api/check-price', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          url: productUrl,
          targetPrice: parseFloat(targetPrice)
        })
      });

      const data = await response.json();

      if (response.ok) {
        if (data.isBelowTarget) {
          setMessage(`🎉 Great news! The price has dropped to ₹${data.currentPrice}! (Target: ₹${targetPrice})`);
          
          // Show OS notification if permission granted
          if (notificationPermission === 'granted') {
            new Notification('Price Alert!', {
              body: `${sliceProductUrl(productUrl)} price dropped to ₹${data.currentPrice}! Target was ₹${targetPrice}`,
              icon: '/favicon.ico'
            });
          }
        } else {
          setMessage(`Current price: ₹${data.currentPrice}. Target price: ₹${targetPrice}. No price drop detected.`);
        }
      } else {
        setMessage(`Error: ${data.message || 'Failed to check price'}`);
      }
    } catch (error) {
      setMessage(`Error: ${error.message}`);
    } finally {
      setLoading(false);
    }
  };

  const startMonitoring = async () => {
    if (!productUrl || !targetPrice) {
      setMessage('Please enter both product URL and target price');
      return;
    }

    // Generate a unique ID for this tracking item
    const trackingId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

    try {
      const response = await fetch('http://localhost:8080/api/track-price', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          url: productUrl,
          targetPrice: parseFloat(targetPrice),
          id: trackingId
        })
      });

      const data = await response.json();

      if (response.ok) {
        setMessage(`✅ Started monitoring ${sliceProductUrl(productUrl)} for price drops below ₹${targetPrice}`);
        loadMonitoredItems();
        setProductUrl('');
        setTargetPrice('');
      } else {
        setMessage(`Error: ${data.message || 'Failed to start monitoring'}`);
      }
    } catch (error) {
      setMessage(`Error: ${error.message}`);
    }
  };

  const stopMonitoring = async (id) => {
    try {
      const response = await fetch('http://localhost:8080/api/untrack-price', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ id })
      });

      const data = await response.json();

      if (response.ok) {
        setMessage(`🛑 Stopped monitoring item`);
        loadMonitoredItems();
      } else {
        setMessage(`Error: ${data.message || 'Failed to stop monitoring'}`);
      }
    } catch (error) {
      setMessage(`Error: ${error.message}`);
    }
  };

  const loadMonitoredItems = async () => {
    try {
      const response = await fetch('http://localhost:8080/api/tracked-items');
      const data = await response.json();

      if (response.ok) {
        setMonitoredItems(data.items || []);
      }
    } catch (error) {
      console.error('Failed to load monitored items:', error);
    }
  };

  // Load monitored items when WebSocket connects
  useEffect(() => {
    if (wsConnected) {
      loadMonitoredItems();
    }
  }, [wsConnected]);

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-blue-50 dark:from-gray-900 dark:to-gray-800 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-2xl mx-auto">
        <div className="text-center mb-12">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-4">
            Price Tracker
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-400">
            Monitor Amazon prices and get notified when they drop
          </p>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8 mb-8">
          <form onSubmit={handleSubmit} className="space-y-6">
            <div>
              <label htmlFor="productUrl" className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
                Product URL
              </label>
              <input
                type="url"
                id="productUrl"
                value={productUrl}
                onChange={(e) => setProductUrl(e.target.value)}
                placeholder="https://www.amazon.in/product-url"
                className="w-full px-4 py-3 border border-gray-200 dark:border-gray-600 rounded-xl shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white transition-all duration-200"
                required
              />
            </div>

            <div>
              <label htmlFor="targetPrice" className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
                Target Price (₹)
              </label>
              <input
                type="number"
                id="targetPrice"
                value={targetPrice}
                onChange={(e) => setTargetPrice(e.target.value)}
                placeholder="Enter target price in rupees"
                step="0.01"
                min="0"
                className="w-full px-4 py-3 border border-gray-200 dark:border-gray-600 rounded-xl shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white transition-all duration-200"
                required
              />
            </div>

            <div className="flex space-x-4">
              <button
                type="submit"
                disabled={loading}
                className="flex-1 flex justify-center py-3 px-6 border border-transparent rounded-xl shadow-sm text-sm font-semibold text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
              >
                {loading ? 'Checking...' : 'Check Now'}
              </button>
              
              <button
                type="button"
                onClick={startMonitoring}
                disabled={!wsConnected || !productUrl || !targetPrice}
                className="flex-1 flex justify-center py-3 px-6 border border-transparent rounded-xl shadow-sm text-sm font-semibold text-white bg-green-600 hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
              >
                Start Monitoring
              </button>
            </div>
          </form>

          {message && (
            <div className={`mt-6 p-4 rounded-xl border ${
              message.includes('Great news') || message.includes('Price Alert')
                ? 'bg-green-50 border-green-200 text-green-800 dark:bg-green-900 dark:border-green-700 dark:text-green-200'
                : message.includes('Error')
                ? 'bg-red-50 border-red-200 text-red-800 dark:bg-red-900 dark:border-red-700 dark:text-red-200'
                : 'bg-blue-50 border-blue-200 text-blue-800 dark:bg-blue-900 dark:border-blue-700 dark:text-blue-200'
            }`}>
              {message}
            </div>
          )}

          {notificationPermission === 'denied' && (
            <div className="mt-6 p-4 bg-yellow-50 border border-yellow-200 text-yellow-800 dark:bg-yellow-900 dark:border-yellow-700 dark:text-yellow-200 rounded-xl">
              <p className="text-sm">
                ⚠️ Notifications are disabled. Enable them in your browser settings to receive price alerts.
              </p>
            </div>
          )}
        </div>

        {/* Monitored Items List */}
        {monitoredItems.length > 0 && (
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white mb-6">
              Monitored Items ({monitoredItems.length})
            </h2>
            <div className="space-y-4">
              {monitoredItems.map((item, index) => (
                <div key={index} className="bg-gray-50 dark:bg-gray-700 rounded-xl p-6 border border-gray-100 dark:border-gray-600">
                  <div className="flex justify-between items-start">
                    <div className="flex-1 min-w-0">
                      <p className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
                        {sliceProductUrl(item.url)}
                      </p>
                      <p className="text-sm text-gray-600 dark:text-gray-400">
                        Target: ₹{item.targetPrice}
                      </p>
                    </div>
                    <button
                      onClick={() => stopMonitoring(item.id)}
                      className="ml-4 px-4 py-2 text-sm bg-red-600 hover:bg-red-700 text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-red-500 transition-all duration-200"
                    >
                      Stop
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* WebSocket Connection Status */}
        <div className="mt-8 text-center">
          <div className={`inline-flex items-center px-4 py-2 rounded-full text-sm font-medium ${
            wsConnected 
              ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
              : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
          }`}>
            <div className={`w-3 h-3 rounded-full mr-3 ${
              wsConnected ? 'bg-green-500' : 'bg-red-500'
            }`}></div>
            {wsConnected ? 'Connected to price monitoring service' : 'Disconnected from monitoring service'}
          </div>
        </div>
      </div>
    </div>
  );
}
