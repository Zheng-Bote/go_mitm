#!/usr/bin/env python3
# -*- coding: utf-8 -*-
#
# SPDX-FileComment: MitM Aggregator Simple Python Prototype
# SPDX-FileType: SOURCE
# SPDX-FileContributor: ZHENG Robert
# SPDX-FileCopyrightText: 2026 ZHENG Robert
# SPDX-License-Identifier: Apache-2.0
#
# @file uploader_simple.py
# @brief Simple Python uploader prototype for API validation.
# @version 0.1.0
# @date 2026-05-10
#
# @author ZHENG Robert (robert @hase-zheng.net)
# @copyright Copyright (c) 2026 ZHENG Robert
# @LICENSE Apache-2.0
#
"""
uploader.py
Flow:
  1) POST /api/refreshtoken  -> returns {"Token": "...", "ExpiryDateTime": "..."}
  2) GET  /api/token/        -> Authorization: Bearer <RefreshToken>
                              returns {"AccessToken": "...", "AccessTokenExpiryDateTime": "..."}
  3) POST /api/employeeimport with Authorization: Bearer <AccessToken> and JSON body
Usage:
    ./uploader_simple.py --base-url <CORITY_BASE_URL> --login <CORITY_LOGIN> --password <CORITY_PASSWORD> --upload-file <CORITY_UPLOAD_FILE>
"""

import argparse
import json
import logging
import os
import sys
import time
from dataclasses import dataclass
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Optional, Dict, Any

import requests
from requests.adapters import HTTPAdapter, Retry

# ---------------------------
# Config
# ---------------------------
APP_NAME = "saas_cli"
DEFAULT_CONFIG_DIR = Path.home() / ".config" / APP_NAME
TOKEN_FILE = DEFAULT_CONFIG_DIR / "token.json"
DEFAULT_TIMEOUT = 15
RETRY_STRATEGY = Retry(total=3, backoff_factor=1, status_forcelist=(429, 500, 502, 503, 504))


# ---------------------------
# Utilities
# ---------------------------
def setup_logging(verbose: bool):
    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(asctime)s %(levelname)s %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )


def requests_session() -> requests.Session:
    s = requests.Session()
    adapter = HTTPAdapter(max_retries=RETRY_STRATEGY)
    s.mount("https://", adapter)
    s.mount("http://", adapter)
    return s


def parse_iso_datetime(s: str) -> Optional[datetime]:
    if not s:
        return None
    try:
        # Python 3.11+ supports fromisoformat with timezone; keep robust
        dt = datetime.fromisoformat(s)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.astimezone(timezone.utc)
    except Exception:
        # fallback common format
        for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M:%S.%f"):
            try:
                dt = datetime.strptime(s, fmt).replace(tzinfo=timezone.utc)
                return dt
            except Exception:
                continue
    return None


@dataclass
class TokenStore:
    path: Path

    def save(self, access_token: str, expiry_iso: Optional[str] = None, refresh_token: Optional[str] = None, refresh_expiry: Optional[str] = None):
        self.path.parent.mkdir(parents=True, exist_ok=True)
        payload: Dict[str, Any] = {
            "access_token": access_token,
            "saved_at": int(time.time())
        }
        if expiry_iso:
            payload["access_expiry"] = expiry_iso
        if refresh_token:
            payload["refresh_token"] = refresh_token
        if refresh_expiry:
            payload["refresh_expiry"] = refresh_expiry
        with open(self.path, "w", encoding="utf-8") as f:
            json.dump(payload, f, indent=2)
        try:
            os.chmod(self.path, 0o600)
        except Exception:
            pass
        logging.info("Token saved to %s", self.path)

    def load(self) -> Optional[Dict[str, Any]]:
        if not self.path.exists():
            return None
        try:
            with open(self.path, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception as e:
            logging.warning("Failed to read token file: %s", e)
            return None

    def clear(self):
        try:
            if self.path.exists():
                self.path.unlink()
                logging.info("Token file removed")
        except Exception as e:
            logging.warning("Failed to remove token file: %s", e)


# ---------------------------
# Authentication steps
# ---------------------------
def refresh_token_request(session: requests.Session, base_url: str, login: str, password: str, timeout: int = DEFAULT_TIMEOUT) -> Dict[str, Optional[str]]:
    """
    POST /api/refreshtoken
    Returns dict with keys: refresh_token (Token), refresh_expiry (ExpiryDateTime or None)
    """
    url = f"{base_url.rstrip('/')}/api/refreshtoken"
    payload = {"user": {"LoginName": login, "Loginpassword": password}}
    headers = {"Content-Type": "application/json"}

    logging.info("Authenticating (refresh token) against %s", url)
    resp = session.post(url, json=payload, headers=headers, timeout=timeout)
    logging.debug("Refresh response status: %s", resp.status_code)
    resp.raise_for_status()
    data = resp.json()
    logging.debug("Refresh response JSON: %s", data)

    refresh_token = None
    refresh_expiry = None
    if isinstance(data, dict):
        refresh_token = data.get("Token") or data.get("token")
        refresh_expiry = data.get("ExpiryDateTime") or data.get("expiry") or data.get("RefreshDateTime")
    if not refresh_token:
        raise ValueError("Refresh token not found in response")
    return {"refresh_token": refresh_token, "refresh_expiry": refresh_expiry}


def get_access_token(session: requests.Session, base_url: str, refresh_token: str, timeout: int = DEFAULT_TIMEOUT) -> Dict[str, Optional[str]]:
    """
    GET /api/token/ with Authorization: Bearer <refresh_token>
    Returns dict with keys: access_token (AccessToken), access_expiry (AccessTokenExpiryDateTime)
    """
    url = f"{base_url.rstrip('/')}/api/token/"
    headers = {"Authorization": f"Bearer {refresh_token}"}
    logging.info("Requesting access token from %s", url)
    resp = session.get(url, headers=headers, timeout=timeout)
    logging.debug("Access token response status: %s", resp.status_code)
    resp.raise_for_status()
    data = resp.json()
    logging.debug("Access token response JSON: %s", data)

    access_token = None
    access_expiry = None
    if isinstance(data, dict):
        access_token = data.get("AccessToken") or data.get("access_token") or data.get("token")
        access_expiry = data.get("AccessTokenExpiryDateTime") or data.get("access_expiry") or data.get("ExpiryDateTime")
    if not access_token:
        raise ValueError("Access token not found in /api/token response")
    return {"access_token": access_token, "access_expiry": access_expiry}


# ---------------------------
# Upload
# ---------------------------
def upload_employee_import(session: requests.Session, base_url: str, access_token: str, payload: Dict[str, Any], timeout: int = DEFAULT_TIMEOUT) -> Dict[str, Any]:
    url = f"{base_url.rstrip('/')}/api/employeeimport"
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {access_token}",
    }
    logging.info("Uploading employee import to %s", url)
    # debug: write body preview
    logging.debug("Request headers: %s", headers)
    body_text = json.dumps(payload, ensure_ascii=False)
    logging.debug("Request body length=%d", len(body_text))
    logging.debug("Request body preview: %s", body_text[:2000])
    resp = session.post(url, data=body_text.encode("utf-8"), headers=headers, timeout=timeout)
    logging.debug("Upload response status: %s", resp.status_code)
    resp.raise_for_status()
    try:
        return resp.json()
    except ValueError:
        return {"raw": resp.text}


# ---------------------------
# CLI
# ---------------------------
def parse_args():
    p = argparse.ArgumentParser(description="Authenticate (refresh->access) and upload employee import to saas API")
    p.add_argument("--base-url", required=True, help="Base URL, e.g. https://mycompanygroup.demo.saas.com")
    p.add_argument("--login", required=True, help="Login name for refresh token")
    p.add_argument("--password", required=True, help="Password for refresh token")
    p.add_argument("--upload-file", required=True, help="Path to JSON file with payload to upload (employeeimport body)")
    p.add_argument("--token-file", default=str(TOKEN_FILE), help=f"Where to store token (default: {TOKEN_FILE})")
    p.add_argument("--force-auth", action="store_true", help="Force re-authentication even if stored access token is valid")
    p.add_argument("--verbose", action="store_true", help="Verbose logging")
    return p.parse_args()


def access_token_valid(stored: Dict[str, Any]) -> bool:
    if not stored:
        return False
    token = stored.get("access_token")
    expiry = stored.get("access_expiry")
    if not token:
        return False
    if not expiry:
        return True
    dt = parse_iso_datetime(expiry)
    if not dt:
        return True
    now = datetime.now(timezone.utc)
    # margin 60s
    return dt > (now + timedelta(seconds=60))


def main():
    args = parse_args()
    setup_logging(args.verbose)

    token_store = TokenStore(Path(args.token_file))
    session = requests_session()

    stored = token_store.load()
    access_token = None
    access_expiry = None
    refresh_token = None
    refresh_expiry = None

    if stored and not args.force_auth:
        access_token = stored.get("access_token")
        access_expiry = stored.get("access_expiry")
        refresh_token = stored.get("refresh_token")
        refresh_expiry = stored.get("refresh_expiry")
        if access_token and access_token_valid(stored):
            logging.info("Using stored access token (valid until %s)", access_expiry)
        else:
            access_token = None  # force refresh below

    # If no valid access token, perform refresh -> get access
    if not access_token:
        try:
            rt = refresh_token_request(session, args.base_url, args.login, args.password)
            refresh_token = rt["refresh_token"]
            refresh_expiry = rt.get("refresh_expiry")
            logging.info("Received refresh token (len=%d)", len(refresh_token))
        except Exception as e:
            logging.error("Failed to obtain refresh token: %s", e)
            sys.exit(2)

        try:
            at = get_access_token(session, args.base_url, refresh_token)
            access_token = at["access_token"]
            access_expiry = at.get("access_expiry")
            logging.info("Received access token (len=%d)", len(access_token))
            token_store.save(access_token=access_token, expiry_iso=access_expiry, refresh_token=refresh_token, refresh_expiry=refresh_expiry)
        except Exception as e:
            logging.error("Failed to obtain access token: %s", e)
            sys.exit(3)

    # Load upload payload
    upload_path = Path(args.upload_file)
    if not upload_path.exists():
        logging.error("Upload file not found: %s", upload_path)
        sys.exit(4)

    try:
        with open(upload_path, "r", encoding="utf-8") as f:
            payload = json.load(f)
    except Exception as e:
        logging.error("Failed to read upload file: %s", e)
        sys.exit(5)

    # Perform upload
    try:
        result = upload_employee_import(session, args.base_url, access_token, payload)
        logging.info("Upload successful. Server response:")
        print(json.dumps(result, indent=2, ensure_ascii=False))
    except requests.HTTPError as e:
        # If 401, try one re-auth cycle (token might be expired)
        resp = getattr(e, "response", None)
        status = resp.status_code if resp is not None else "?"
        text = resp.text if resp is not None else str(e)
        logging.error("Upload failed: %s - %s", status, text)
        if status == 401:
            logging.info("Access token rejected; attempting one re-auth and retry")
            try:
                rt = refresh_token_request(session, args.base_url, args.login, args.password)
                refresh_token = rt["refresh_token"]
                at = get_access_token(session, args.base_url, refresh_token)
                access_token = at["access_token"]
                access_expiry = at.get("access_expiry")
                token_store.save(access_token=access_token, expiry_iso=access_expiry, refresh_token=refresh_token, refresh_expiry=rt.get("refresh_expiry"))
                result = upload_employee_import(session, args.base_url, access_token, payload)
                logging.info("Retry upload successful. Server response:")
                print(json.dumps(result, indent=2, ensure_ascii=False))
                return
            except Exception as e2:
                logging.error("Retry failed: %s", e2)
        sys.exit(6)
    except Exception as e:
        logging.error("Upload failed: %s", e)
        sys.exit(7)


if __name__ == "__main__":
    main()
