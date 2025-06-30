// src/App.jsx
import React, { useState, useEffect } from 'react';

// VAPID public key (generate this once for your backend)
// You can generate VAPID keys using npx web-push generate-vapid-keys
const VAPID_PUBLIC_KEY = 'YOUR_VAPID_PUBLIC_KEY_HERE'; // Replace with your actual key

function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - base64String.length % 4) % 4);
  const base64 = (base64String + padding)
    .replace(/-/g, '+')
    .replace(/_/g, '/');
  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

function App() {
  const [productUrl, setProductUrl] = useState('');
  const [targetPrice, setTargetPrice] = useState('');
  const [status, setStatus] = useState('');
  const [isSubscribed, setIsSubscribed] = useState(false);
  const [subscription, setSubscription] = useState(null);

  useEffect(() => {
    // Check for existing subscription when component mounts
    if ('serviceWorker' in navigator && 'PushManager' in window) {
      navigator.serviceWorker.ready.then(registration => {
        registration.pushManager.getSubscription().then(sub => {
          if (sub) {
            setIsSubscribed(true);
            setSubscription(sub);
            console.log('User IS subscribed.');
          } else {
            setIsSubscribed(false);
            console.log('User is NOT subscribed.');
          }
        });
      });
    }
  }, []);

  const handleSubscribe = async () => {
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
      setStatus('Push notifications not supported by this browser.');
      return;
    }
    try {
      const registration = await navigator.serviceWorker.register('/service-worker.js');
      await navigator.serviceWorker.ready; // Ensure service worker is active

      const sub = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(VAPID_PUBLIC_KEY),
      });

      setIsSubscribed(true);
      setSubscription(sub);
      setStatus('Subscribed to push notifications!');
      console.log('User subscribed:', sub);

      // Optional: Send the subscription object to your backend immediately
      // await fetch('/api/subscribe', {
      //   method: 'POST',
      //   body: JSON.stringify(sub),
      //   headers: { 'Content-Type': 'application/json' },
      // });

    } catch (error) {
      console.error('Failed to subscribe:', error);
      setStatus(`Failed to subscribe: ${error.message}`);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!productUrl || !targetPrice) {
      setStatus('Please enter URL and target price.');
      return;
    }
    if (!isSubscribed || !subscription) {
      setStatus('Please subscribe to notifications first.');
      return;
    }

    setStatus('Sending request to track price...');
    try {
      const response = await fetch('/api/track', { // We'll proxy this in vite.config.js
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          url: productUrl,
          threshold: parseFloat(targetPrice),
          subscription: subscription, // Send subscription object
        }),
      });
      const data = await response.json();
      if (response.ok) {
        setStatus(`Tracking started for: ${productUrl.substring(0,30)}... Price: ${data.currentPrice || 'N/A'}`);
        setProductUrl('');
        setTargetPrice('');
      } else {
        setStatus(`Error: ${data.error || 'Failed to start tracking'}`);
      }
    } catch (error) {
      console.error('Error tracking price:', error);
      setStatus('Error connecting to backend.');
    }
  };

  return (
    <div className="min-h-screen bg-gray-900 text-white flex flex-col items-center justify-center p-4">
      <div className="bg-gray-800 p-8 rounded-lg shadow-xl w-full max-w-md">
        <h1 className="text-3xl font-bold mb-6 text-center text-indigo-400">Price Tracker</h1>

        {!isSubscribed && (
          <button
            onClick={handleSubscribe}
            className="w-full bg-green-500 hover:bg-green-600 text-white font-bold py-3 px-4 rounded-md focus:outline-none focus:shadow-outline mb-6 transition duration-150"
          >
            Enable Notifications
          </button>
        )}
        {isSubscribed && <p className="text-center text-green-400 mb-4">Notifications Enabled!</p>}


        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <label htmlFor="productUrl" className="block text-sm font-medium text-gray-300">
              Product URL
            </label>
            <input
              type="url"
              id="productUrl"
              value={productUrl}
              onChange={(e) => setProductUrl(e.target.value)}
              className="mt-1 block w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md shadow-sm placeholder-gray-500 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              placeholder="https://www.example.com/product/123"
              required
            />
          </div>
          <div>
            <label htmlFor="targetPrice" className="block text-sm font-medium text-gray-300">
              Target Price
            </label>
            <input
              type="number"
              id="targetPrice"
              value={targetPrice}
              onChange={(e) => setTargetPrice(e.target.value)}
              className="mt-1 block w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md shadow-sm placeholder-gray-500 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
              placeholder="29.99"
              step="0.01"
              required
            />
          </div>
          <button
            type="submit"
            disabled={!isSubscribed}
            className={`w-full font-semibold py-3 px-4 rounded-md focus:outline-none focus:shadow-outline transition duration-150 ${
              isSubscribed
                ? 'bg-indigo-600 hover:bg-indigo-700'
                : 'bg-gray-500 cursor-not-allowed'
            }`}
          >
            Track Price
          </button>
        </form>
        {status && <p className="mt-6 text-center text-sm text-gray-400">{status}</p>}
      </div>
      <p className="text-xs text-gray-500 mt-8">Note: Tracking stops if the backend server restarts.</p>
    </div>
  );
}

export default App;