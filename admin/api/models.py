from django.db import models
from django.db.models import F, Q
from django.utils import timezone


class Person(models.Model):
    name = models.CharField(max_length=255, unique=True)
    birthyear = models.PositiveIntegerField(blank=True, null=True)
    birthplace = models.CharField(max_length=255, blank=True, null=True)
    bio = models.TextField(blank=True, null=True)

    class Meta:
        ordering = ['name']

    def __str__(self):
        return self.name


class ParentChild(models.Model):
    parent = models.ForeignKey(Person, related_name='child_links', on_delete=models.CASCADE)
    child = models.ForeignKey(Person, related_name='parent_links', on_delete=models.CASCADE)

    class Meta:
        constraints = [
            models.UniqueConstraint(fields=['parent', 'child'], name='unique_parent_child'),
            models.CheckConstraint(condition=~Q(parent=F('child')), name='parent_is_not_child'),
        ]

    def __str__(self):
        return f'{self.parent} -> {self.child}'


class Marriage(models.Model):
    person_a = models.ForeignKey(Person, related_name='marriages_as_a', on_delete=models.CASCADE)
    person_b = models.ForeignKey(Person, related_name='marriages_as_b', on_delete=models.CASCADE)

    class Meta:
        constraints = [
            models.UniqueConstraint(fields=['person_a', 'person_b'], name='unique_marriage'),
            models.CheckConstraint(condition=Q(person_a__lt=F('person_b')), name='marriage_canonical_order'),
        ]

    def __str__(self):
        return f'{self.person_a} + {self.person_b}'


class Location(models.Model):
    person = models.OneToOneField(Person, related_name='location', on_delete=models.CASCADE)
    name = models.CharField(max_length=255)
    lat = models.FloatField(blank=True, null=True)
    lng = models.FloatField(blank=True, null=True)

    def __str__(self):
        return f'{self.name} ({self.lat}, {self.lng})'


class Photo(models.Model):
    person = models.ForeignKey(Person, related_name='photos', on_delete=models.CASCADE)
    description = models.CharField(max_length=255, blank=True, null=True)
    file_path = models.CharField(max_length=255, blank=True, null=True)
    profile_pic = models.BooleanField(default=False)
    rotation = models.IntegerField(default=0)

    def __str__(self):
        return f'{self.description} ({"Profile Pic" if self.profile_pic else "Photo"})'

    def save(self, *args, **kwargs):
        if self.profile_pic:
            others = Photo.objects.filter(person=self.person, profile_pic=True)
            if self.pk:
                others = others.exclude(pk=self.pk)
            others.update(profile_pic=False)
        super().save(*args, **kwargs)


class History(models.Model):
    created_at = models.DateTimeField(default=timezone.now)
    username = models.CharField(max_length=100)
    action = models.CharField(max_length=100)
    recipient = models.CharField(max_length=100)

    class Meta:
        ordering = ['-created_at']
        indexes = [models.Index(fields=['-created_at'])]

    def __str__(self):
        return f'{self.username} {self.action} {self.recipient}'
