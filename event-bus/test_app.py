import json

import pytest

from app import app


@pytest.fixture
def client():
    app.config["TESTING"] = True
    with app.test_client() as client:
        yield client


@pytest.fixture(autouse=True)
def reset_state():
    from app import channels, subscribers, event_log

    channels.clear()
    subscribers.clear()
    event_log.clear()


def test_health(client):
    resp = client.get("/health")
    assert resp.status_code == 200
    data = resp.get_json()
    assert data["status"] == "healthy"
    assert data["service"] == "event-bus"
    assert "timestamp" in data


def test_create_channel(client):
    resp = client.post("/channels/test-channel")
    assert resp.status_code == 201
    data = resp.get_json()
    assert data["channel"] == "test-channel"
    assert data["status"] == "created"


def test_create_channel_idempotent(client):
    client.post("/channels/test-channel")
    resp = client.post("/channels/test-channel")
    assert resp.status_code == 201


def test_list_channels(client):
    client.post("/channels/ch1")
    client.post("/channels/ch2")
    resp = client.get("/channels")
    assert resp.status_code == 200
    data = resp.get_json()
    assert "ch1" in data["channels"]
    assert "ch2" in data["channels"]


def test_publish_event(client):
    client.post("/channels/orders")
    resp = client.post(
        "/publish/orders",
        data=json.dumps({"item": "widget", "qty": 5}),
        content_type="application/json",
    )
    assert resp.status_code == 201
    data = resp.get_json()
    assert data["event"]["channel"] == "orders"
    assert data["event"]["payload"]["item"] == "widget"
    assert "id" in data["event"]
    assert "timestamp" in data["event"]


def test_publish_to_nonexistent_channel(client):
    resp = client.post(
        "/publish/nonexistent",
        data=json.dumps({"item": "widget"}),
        content_type="application/json",
    )
    assert resp.status_code == 404


def test_publish_invalid_json(client):
    client.post("/channels/orders")
    resp = client.post(
        "/publish/orders",
        data="not json",
        content_type="text/plain",
    )
    assert resp.status_code == 400


def test_subscribe(client):
    client.post("/channels/orders")
    resp = client.post(
        "/subscribe",
        data=json.dumps({"channel": "orders", "callback_url": "http://example.com/hook"}),
        content_type="application/json",
    )
    assert resp.status_code == 201
    data = resp.get_json()
    assert data["subscriber"]["channel"] == "orders"
    assert "id" in data["subscriber"]


def test_subscribe_missing_channel(client):
    resp = client.post(
        "/subscribe",
        data=json.dumps({}),
        content_type="application/json",
    )
    assert resp.status_code == 400


def test_subscribe_nonexistent_channel(client):
    resp = client.post(
        "/subscribe",
        data=json.dumps({"channel": "missing"}),
        content_type="application/json",
    )
    assert resp.status_code == 404


def test_list_subscribers(client):
    client.post("/channels/orders")
    client.post(
        "/subscribe",
        data=json.dumps({"channel": "orders"}),
        content_type="application/json",
    )
    resp = client.get("/subscribers")
    assert resp.status_code == 200
    data = resp.get_json()
    assert len(data["subscribers"]) == 1


def test_list_events(client):
    client.post("/channels/orders")
    client.post(
        "/publish/orders",
        data=json.dumps({"item": "a"}),
        content_type="application/json",
    )
    client.post(
        "/publish/orders",
        data=json.dumps({"item": "b"}),
        content_type="application/json",
    )
    resp = client.get("/events")
    assert resp.status_code == 200
    data = resp.get_json()
    assert data["total"] == 2


def test_list_events_with_channel_filter(client):
    client.post("/channels/orders")
    client.post("/channels/users")
    client.post(
        "/publish/orders",
        data=json.dumps({"item": "a"}),
        content_type="application/json",
    )
    client.post(
        "/publish/users",
        data=json.dumps({"name": "alice"}),
        content_type="application/json",
    )
    resp = client.get("/events?channel=orders")
    data = resp.get_json()
    assert data["total"] == 1
    assert data["events"][0]["channel"] == "orders"


def test_channel_events(client):
    client.post("/channels/orders")
    client.post(
        "/publish/orders",
        data=json.dumps({"item": "x"}),
        content_type="application/json",
    )
    resp = client.get("/events/orders")
    assert resp.status_code == 200
    data = resp.get_json()
    assert data["channel"] == "orders"
    assert data["total"] == 1


def test_channel_events_not_found(client):
    resp = client.get("/events/missing")
    assert resp.status_code == 404


def test_stats(client):
    client.post("/channels/ch1")
    client.post(
        "/publish/ch1",
        data=json.dumps({"a": 1}),
        content_type="application/json",
    )
    client.post(
        "/subscribe",
        data=json.dumps({"channel": "ch1"}),
        content_type="application/json",
    )
    resp = client.get("/stats")
    assert resp.status_code == 200
    data = resp.get_json()
    assert data["total_events"] == 1
    assert data["total_channels"] == 1
    assert data["total_subscribers"] == 1
