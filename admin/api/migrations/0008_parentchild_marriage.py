import django.db.models.deletion
from django.db import migrations, models


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0007_person_birthyear'),
    ]

    operations = [
        migrations.CreateModel(
            name='Marriage',
            fields=[
                ('id', models.BigAutoField(auto_created=True, primary_key=True, serialize=False, verbose_name='ID')),
            ],
        ),
        migrations.CreateModel(
            name='ParentChild',
            fields=[
                ('id', models.BigAutoField(auto_created=True, primary_key=True, serialize=False, verbose_name='ID')),
            ],
        ),
        migrations.AddField(
            model_name='marriage',
            name='person_a',
            field=models.ForeignKey(on_delete=django.db.models.deletion.CASCADE, related_name='marriages_as_a', to='api.person'),
        ),
        migrations.AddField(
            model_name='marriage',
            name='person_b',
            field=models.ForeignKey(on_delete=django.db.models.deletion.CASCADE, related_name='marriages_as_b', to='api.person'),
        ),
        migrations.AddField(
            model_name='parentchild',
            name='child',
            field=models.ForeignKey(on_delete=django.db.models.deletion.CASCADE, related_name='parent_links', to='api.person'),
        ),
        migrations.AddField(
            model_name='parentchild',
            name='parent',
            field=models.ForeignKey(on_delete=django.db.models.deletion.CASCADE, related_name='child_links', to='api.person'),
        ),
        migrations.AddConstraint(
            model_name='marriage',
            constraint=models.UniqueConstraint(fields=('person_a', 'person_b'), name='unique_marriage'),
        ),
        migrations.AddConstraint(
            model_name='marriage',
            constraint=models.CheckConstraint(condition=models.Q(('person_a__lt', models.F('person_b'))), name='marriage_canonical_order'),
        ),
        migrations.AddConstraint(
            model_name='parentchild',
            constraint=models.UniqueConstraint(fields=('parent', 'child'), name='unique_parent_child'),
        ),
        migrations.AddConstraint(
            model_name='parentchild',
            constraint=models.CheckConstraint(condition=models.Q(('parent', models.F('child')), _negated=True), name='parent_is_not_child'),
        ),
    ]
