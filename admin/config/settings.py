import os
import sys
from pathlib import Path
from urllib.parse import urlparse

import dj_database_url
from dotenv import load_dotenv

BASE_DIR = Path(__file__).resolve().parent.parent

load_dotenv(BASE_DIR.parent / '.env')

SECRET_KEY = os.environ.get('SECRET_KEY', 'django-insecure-local-admin-only')
DEBUG = os.environ.get('DEBUG', 'True') == 'True'
ALLOWED_HOSTS = ['127.0.0.1', 'localhost']

INSTALLED_APPS = [
    'django.contrib.admin',
    'django.contrib.auth',
    'django.contrib.contenttypes',
    'django.contrib.sessions',
    'django.contrib.messages',
    'django.contrib.staticfiles',
    'api',
]

MIDDLEWARE = [
    'django.middleware.security.SecurityMiddleware',
    'django.contrib.sessions.middleware.SessionMiddleware',
    'django.middleware.common.CommonMiddleware',
    'django.middleware.csrf.CsrfViewMiddleware',
    'django.contrib.auth.middleware.AuthenticationMiddleware',
    'django.contrib.messages.middleware.MessageMiddleware',
    'django.middleware.clickjacking.XFrameOptionsMiddleware',
]

ROOT_URLCONF = 'config.urls'

TEMPLATES = [
    {
        'BACKEND': 'django.template.backends.django.DjangoTemplates',
        'DIRS': [],
        'APP_DIRS': True,
        'OPTIONS': {
            'context_processors': [
                'django.template.context_processors.request',
                'django.contrib.auth.context_processors.auth',
                'django.contrib.messages.context_processors.messages',
            ],
        },
    },
]

WSGI_APPLICATION = 'config.wsgi.application'

DATABASE_URL = os.environ['DATABASE_URL']

DATABASES = {'default': dj_database_url.parse(DATABASE_URL)}

print(f'[admin] database: {DATABASE_URL.rsplit("@", 1)[-1]}')

# load_dotenv() will not override an already-exported DATABASE_URL, so any
# shell that has sourced .env.prod points manage.py at Prod DB. These
# commands overwrite or delete rows -- and the fixtures carry explicit primary
# keys, so loaddata would clobber the real people sitting on ids 1-12.
LOCAL_DB_HOSTS = {'localhost', '127.0.0.1', '::1', 'db'}
DESTRUCTIVE_COMMANDS = {'loaddata', 'flush', 'sqlflush'}

_command = sys.argv[1] if len(sys.argv) > 1 else ''
_db_host = urlparse(DATABASE_URL).hostname or ''

if _command in DESTRUCTIVE_COMMANDS and _db_host not in LOCAL_DB_HOSTS:
    raise SystemExit(
        f'\n[admin] refusing to run "{_command}" against non-local database '
        f'host "{_db_host}".\n'
        f'        Source .env (local), not .env.prod.\n'
    )

AUTH_PASSWORD_VALIDATORS = [
    {'NAME': 'django.contrib.auth.password_validation.UserAttributeSimilarityValidator'},
    {'NAME': 'django.contrib.auth.password_validation.MinimumLengthValidator'},
    {'NAME': 'django.contrib.auth.password_validation.CommonPasswordValidator'},
    {'NAME': 'django.contrib.auth.password_validation.NumericPasswordValidator'},
]

LANGUAGE_CODE = 'en-us'
TIME_ZONE = 'UTC'
USE_I18N = True
USE_TZ = True

STATIC_URL = 'static/'
STATIC_ROOT = BASE_DIR / 'staticfiles'

DEFAULT_AUTO_FIELD = 'django.db.models.BigAutoField'
