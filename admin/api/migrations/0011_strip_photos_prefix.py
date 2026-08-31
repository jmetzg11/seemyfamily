from django.db import migrations
from django.db.models import Value
from django.db.models.functions import Concat, Substr

PREFIX = 'photos/'


def strip(apps, schema_editor):
    Photo = apps.get_model('api', 'Photo')

    Photo.objects.filter(file_path__startswith=PREFIX).update(
        file_path=Substr('file_path', len(PREFIX) + 1)
    )


def restore(apps, schema_editor):
    Photo = apps.get_model('api', 'Photo')

    Photo.objects.filter(file_path__isnull=False).update(
        file_path=Concat(Value(PREFIX), 'file_path')
    )


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0010_drop_relations_and_visitor'),
    ]

    operations = [
        migrations.RunPython(strip, restore),
    ]
