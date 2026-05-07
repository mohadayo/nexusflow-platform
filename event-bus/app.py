import logging
import os
import time
import uuid
from collections import defaultdict
from threading import Lock

from flask import Flask, jsonify, request
from flask_cors import CORS

app = Flask(__name__)
CORS(app)

LOG_LEVEL = os.environ.get("LOG_LEVEL", "INFO").upper()
logging.basicConfig(
    level=getattr(logging, LOG_LEVEL, logging.INFO),
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("event-bus")

channels = defaultdict(list)
subscribers = {}
event_log = []
lock = Lock()

MAX_EVENT_LOG = int(os.environ.get("MAX_EVENT_LOG", "1000"))


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "service": "event-bus", "timestamp": time.time()})


@app.route("/channels", methods=["GET"])
def list_channels():
    with lock:
        result = {ch: len(events) for ch, events in channels.items()}
    logger.info("Listed %d channels", len(result))
    return jsonify({"channels": result})


@app.route("/channels/<channel_name>", methods=["POST"])
def create_channel(channel_name):
    if not channel_name or not channel_name.strip():
        logger.warning("Attempt to create channel with empty name")
        return jsonify({"error": "Channel name cannot be empty"}), 400
    with lock:
        if channel_name not in channels:
            channels[channel_name] = []
            logger.info("Channel created: %s", channel_name)
        else:
            logger.info("Channel already exists: %s", channel_name)
    return jsonify({"channel": channel_name, "status": "created"}), 201


@app.route("/publish/<channel_name>", methods=["POST"])
def publish(channel_name):
    with lock:
        if channel_name not in channels:
            logger.warning("Publish to non-existent channel: %s", channel_name)
            return jsonify({"error": f"Channel '{channel_name}' not found"}), 404

    data = request.get_json(silent=True)
    if data is None:
        logger.warning("Invalid JSON payload for publish to %s", channel_name)
        return jsonify({"error": "Request body must be valid JSON"}), 400

    event = {
        "id": str(uuid.uuid4()),
        "channel": channel_name,
        "payload": data,
        "timestamp": time.time(),
    }

    with lock:
        channels[channel_name].append(event)
        event_log.append(event)
        if len(event_log) > MAX_EVENT_LOG:
            event_log.pop(0)

    logger.info("Event %s published to channel %s", event["id"], channel_name)
    return jsonify({"event": event}), 201


@app.route("/subscribe", methods=["POST"])
def subscribe():
    data = request.get_json(silent=True)
    if not data or "channel" not in data:
        return jsonify({"error": "Missing 'channel' in request body"}), 400

    channel_name = data["channel"]
    callback_url = data.get("callback_url", "")
    subscriber_id = str(uuid.uuid4())

    with lock:
        if channel_name not in channels:
            return jsonify({"error": f"Channel '{channel_name}' not found"}), 404
        subscribers[subscriber_id] = {
            "id": subscriber_id,
            "channel": channel_name,
            "callback_url": callback_url,
            "created_at": time.time(),
        }

    logger.info("Subscriber %s registered for channel %s", subscriber_id, channel_name)
    return jsonify({"subscriber": subscribers[subscriber_id]}), 201


@app.route("/subscribers", methods=["GET"])
def list_subscribers():
    with lock:
        result = list(subscribers.values())
    return jsonify({"subscribers": result})


@app.route("/events", methods=["GET"])
def list_events():
    limit = request.args.get("limit", 50, type=int)
    channel = request.args.get("channel", None)
    with lock:
        filtered = event_log
        if channel:
            filtered = [e for e in event_log if e["channel"] == channel]
        result = filtered[-limit:]
    return jsonify({"events": result, "total": len(result)})


@app.route("/events/<channel_name>", methods=["GET"])
def channel_events(channel_name):
    with lock:
        if channel_name not in channels:
            return jsonify({"error": f"Channel '{channel_name}' not found"}), 404
        result = list(channels[channel_name])
    return jsonify({"channel": channel_name, "events": result, "total": len(result)})


@app.route("/stats", methods=["GET"])
def stats():
    with lock:
        total_events = len(event_log)
        total_channels = len(channels)
        total_subscribers = len(subscribers)
    return jsonify({
        "total_events": total_events,
        "total_channels": total_channels,
        "total_subscribers": total_subscribers,
    })


if __name__ == "__main__":
    port = int(os.environ.get("EVENT_BUS_PORT", "5001"))
    debug = os.environ.get("FLASK_DEBUG", "false").lower() == "true"
    logger.info("Starting Event Bus on port %d", port)
    app.run(host="0.0.0.0", port=port, debug=debug)
