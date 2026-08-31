import django.utils.timezone
from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0009_backfill_relations'),
    ]

    operations = [
        migrations.DeleteModel(
            name='Visitor',
        ),
        migrations.RemoveField(
            model_name='person',
            name='relations',
        ),
        migrations.AlterModelOptions(
            name='history',
            options={'ordering': ['-created_at']},
        ),
        migrations.AlterModelOptions(
            name='person',
            options={'ordering': ['name']},
        ),
        migrations.AlterField(
            model_name='history',
            name='created_at',
            field=models.DateTimeField(default=django.utils.timezone.now),
        ),
        migrations.AlterField(
            model_name='photo',
            name='file_path',
            field=models.CharField(blank=True, max_length=255, null=True),
        ),
        migrations.AddIndex(
            model_name='history',
            index=models.Index(fields=['-created_at'], name='api_history_created_c0cee3_idx'),
        ),
    ]
